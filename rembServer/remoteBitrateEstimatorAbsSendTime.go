package rembServer

import (
	"container/list"
	"github.com/pion/rtp"
	"github.com/pion/utils"
	"github.com/pion/webrtc/deque"
	"github.com/pion/webrtc/mylog"
	"github.com/pion/webrtc/rateCalculator"
	"github.com/pion/webrtc/uvtime"
	"math"
)

const (
	BITRATE_UPDATED = iota
	NO_UPDATE
)
const TimestampGroupLengthMs = 5
const AbsSendTimeFraction = 18
const AbsSendTimeInterArrivalUpshift = 8
const InterArrivalShift = AbsSendTimeFraction + AbsSendTimeInterArrivalUpshift
const InitialProbingIntervalMs = 2000
const MinClusterSize = 4
const MaxProbePackets = 15
const ExpectedNumberOfProbes = 3
const TimestampToMs = 1000.0 / float64(1<<InterArrivalShift)

type Probe struct {
	SendTimeMs  int64
	RecvTimeMs  int64
	PayloadSize uint
}

func NewProbe(SendTimeMs int64, RecvTimeMs int64, PayloadSize uint) Probe {
	node := Probe{}
	node.SendTimeMs = SendTimeMs
	node.RecvTimeMs = RecvTimeMs
	node.PayloadSize = PayloadSize
	return node
}

type Cluster struct {
	SendMeanMs float64
	RecvMeanMs float64

	MeanSize         uint
	Count            int
	NumAboveMinDelta int
}

func (this *Cluster) GetSendBitrateBps() int {
	return int(float64(this.MeanSize) * 8 * 1000 / this.SendMeanMs)
}

func (this *Cluster) GetRecvBitrateBps() int {
	return int(float64(this.MeanSize) * 8 * 1000 / this.RecvMeanMs)
}

type RemoteBitrateEstimatorAbsSendTime struct {
	RemoteBitrateEstimator
	interArrival               *InterArrival
	estimator                  *OveruseEstimator
	detector                   OveruseDetector
	incomingBitrate            rateCalculator.RateCalculator
	incomingBitrateInitialized bool
	recentPropagationDeltaMs   deque.Deque
	recentUpdateTimeMs         deque.Deque
	probes                     *list.List
	totalProbesReceived        int64
	firstPacketTimeMs          int64
	lastUpdateMs               int64
	umaRecorded                bool
	ssrcs                      map[uint32]int64
	remoteRate                 AimdRateControl
}

func NewRemoteBitrateEstimatorAbsSendTime(listener Listener) *RemoteBitrateEstimatorAbsSendTime {
	node := RemoteBitrateEstimatorAbsSendTime{}
	node.listener = listener
	node.probes = list.New()
	node.firstPacketTimeMs = -1
	node.lastUpdateMs = -1
	node.umaRecorded = false
	node.incomingBitrateInitialized = false
	node.incomingBitrate = rateCalculator.NewRateCalculator(0,0)
	node.detector = NewOveruseDetector()
	node.remoteRate = NewAimdRateControl()
	//node,interArrival = NewInterArrival()
	node.ssrcs = make(map[uint32]int64)

	return &node

}

func (this *RemoteBitrateEstimatorAbsSendTime)Close(){
	this.interArrival = nil
	this.estimator = nil
	this.recentPropagationDeltaMs.Clear()
	this.recentUpdateTimeMs.Clear()
	this.probes = nil
	this.ssrcs = nil

}

func (this *RemoteBitrateEstimatorAbsSendTime) Process() {

}
func (this *RemoteBitrateEstimatorAbsSendTime) OnRttUpdate(avgRttMs int64, maxRttMs int64) {
	this.remoteRate.SetRtt(avgRttMs)
}

func (this *RemoteBitrateEstimatorAbsSendTime) RemoveStream(ssrc uint32) {
	delete(this.ssrcs, ssrc)
}

func (this *RemoteBitrateEstimatorAbsSendTime) SetMinBitrate(minBitrateBps int) {
	this.remoteRate.SetMinBitrate(minBitrateBps)
}

func (this *RemoteBitrateEstimatorAbsSendTime) IsWithinClusterBounds(sendDeltaMs int, clusterAggregate *Cluster) bool {
	if (clusterAggregate.Count == 0) {
		return true
	}

	clusterMean := float64(clusterAggregate.SendMeanMs) / float64(clusterAggregate.Count)

	return math.Abs(float64(sendDeltaMs)-clusterMean) < 2.5
}

func (this *RemoteBitrateEstimatorAbsSendTime) AddCluster(clusters *list.List, cluster *Cluster) {
	cluster.SendMeanMs /= float64(cluster.Count)
	cluster.RecvMeanMs /= float64(cluster.Count)
	cluster.MeanSize /= uint(cluster.Count)

	clusters.PushBack(*cluster)
}

func (this *RemoteBitrateEstimatorAbsSendTime) ComputeClusters(clusters *list.List) {
	var current Cluster = Cluster{}
	prevSendTime := int64(-1)
	prevRecvTime := int64(-1)

	for it := this.probes.Front(); nil != it; it = it.Next() {
		probe := it.Value.(Probe)
		if (prevSendTime >= 0) {
			sendDeltaMs := probe.SendTimeMs - prevSendTime
			recvDeltaMs := probe.RecvTimeMs - prevRecvTime

			if (sendDeltaMs >= 1 && recvDeltaMs >= 1) {
				current.NumAboveMinDelta++
			}

			if (!this.IsWithinClusterBounds(int(sendDeltaMs), &current)) {
				if (current.Count >= MinClusterSize) {
					this.AddCluster(clusters, &current)
				}

				current = Cluster{}
			}

			current.SendMeanMs += float64(sendDeltaMs)
			current.RecvMeanMs += float64(recvDeltaMs)
			current.MeanSize += probe.PayloadSize
			current.Count++
		}

		prevSendTime = probe.SendTimeMs
		prevRecvTime = probe.RecvTimeMs
	}

	if (current.Count >= MinClusterSize) {
		this.AddCluster(clusters, &current)
	}

}

func (this *RemoteBitrateEstimatorAbsSendTime) FindBestProbe(clusters *list.List) (bestIt *list.Element) {
	highestProbeBitrateBps := int64(0)
	//mylog.Logger.Infof("miaobinwei\n")
	for it := clusters.Front(); nil != it; it = it.Next() {
		cluster := it.Value.(Cluster)
		if (cluster.SendMeanMs == 0 || cluster.RecvMeanMs == 0) {
			continue
		}
		if (
			cluster.NumAboveMinDelta > cluster.Count/2 &&
				(cluster.RecvMeanMs-cluster.SendMeanMs <= 2.0 && cluster.SendMeanMs-cluster.RecvMeanMs <= 5.0)) {
			probeBitrateBps := utils.MinMy(int64(cluster.GetSendBitrateBps()), int64(cluster.GetRecvBitrateBps()))

			if (probeBitrateBps > highestProbeBitrateBps) {
				highestProbeBitrateBps = probeBitrateBps
				bestIt = it
			}
		} else
		{
			break
		}
	}

	return
}

func (this *RemoteBitrateEstimatorAbsSendTime) ProcessClusters(nowMs int64) int {
	clusters := list.New()
	this.ComputeClusters(clusters)

	if (0 == clusters.Len()) {
		// If we reach the max number of probe packets and still have no clusters,
		// we will remove the oldest one.
		if (this.probes.Len() >= MaxProbePackets) {
			this.probes.Remove(this.probes.Front())
			return NO_UPDATE
		}
	}

	bestIt := this.FindBestProbe(clusters)
	if (nil != bestIt) {
		cluster := bestIt.Value.(Cluster)
		probeBitrateBps := utils.MinMy(int64(cluster.GetSendBitrateBps()), int64(cluster.GetRecvBitrateBps()))

		// Make sure that a probe sent on a lower bitrate than our estimate can't
		// reduce the estimate.
		if (this.IsBitrateImproving(int(probeBitrateBps))) {
			mylog.Logger.Infof(
				"probe successful, sent at %d bps, received at %d bps [mean send delta:%fms, mean recv delta:%f ms, num probes:%d",
				cluster.GetSendBitrateBps(),
				cluster.GetRecvBitrateBps(),
				cluster.SendMeanMs,
				cluster.RecvMeanMs,
				cluster.Count)

			this.remoteRate.SetEstimate(int(probeBitrateBps), nowMs)

			return BITRATE_UPDATED
		}
	}

	if (clusters.Len() >= ExpectedNumberOfProbes) {
		this.probes = list.New()
	}

	return NO_UPDATE
}

func (this *RemoteBitrateEstimatorAbsSendTime) IsBitrateImproving(newBitrateBps int) bool {
	initialProbe := !this.remoteRate.ValidEstimate() && newBitrateBps > 0
	bitrateAboveEstimate := this.remoteRate.ValidEstimate() &&
		newBitrateBps > int(this.remoteRate.LatestEstimate())

	return initialProbe || bitrateAboveEstimate
}

func (this *RemoteBitrateEstimatorAbsSendTime) IncomingPacket(arrivalTimeMs int64, payloadSize uint, packet *rtp.Packet, absSendTime uint32) {
	this.IncomingPacketInfo(arrivalTimeMs, absSendTime, payloadSize, packet.SSRC)
}

func (this *RemoteBitrateEstimatorAbsSendTime) IncomingPacketInfo(arrivalTimeMs int64, sendTime24bits uint32, payloadSize uint, ssrc uint32) {
	if (!this.umaRecorded) {
		this.umaRecorded = true
	}

	timestamp := uint32(sendTime24bits << AbsSendTimeInterArrivalUpshift)
	sendTimeMs := int64(float64(timestamp) * TimestampToMs)
	nowMs := uvtime.GettimeMs()

	incomingBitrate := this.incomingBitrate.GetRate(uint64(arrivalTimeMs))
	if (incomingBitrate != 0) {
		this.incomingBitrateInitialized = true
	} else if (this.incomingBitrateInitialized) {
		// Incoming bitrate had a previous valid value, but now not enough data
		// point are left within the current window. Reset incoming bitrate
		// estimator so that the window size will only contain new data points.
		this.incomingBitrate.Reset()
		this.incomingBitrateInitialized = false
	}

	this.incomingBitrate.Update(uint64(payloadSize), uint64(arrivalTimeMs))

	if (this.firstPacketTimeMs == -1) {
		this.firstPacketTimeMs = nowMs
	}

	tsDelta := uint32(0)
	tDelta := int64(0)
	sizeDelta := int(0)
	updateEstimate := false
	targetBitrateBps := uint32(0)
	var ssrcs deque.Deque

	{
		this.TimeoutStreams(nowMs)

		// MS_ASSERT(this->interArrival.get());
		// MS_ASSERT(this->estimator.get());

		this.ssrcs[ssrc] = nowMs

		// For now only try to detect probes while we don't have a valid estimate.
		// We currently assume that only packets larger than 200 bytes are paced by
		// the sender.
		minProbePacketSize := uint(200)

		if (
			payloadSize > minProbePacketSize &&
				(!this.remoteRate.ValidEstimate() ||
					nowMs-this.firstPacketTimeMs < InitialProbingIntervalMs)) {
			this.probes.PushBack(Probe{sendTimeMs, arrivalTimeMs, payloadSize})
			this.totalProbesReceived++

			// Make sure that a probe which updated the bitrate immediately has an
			// effect by calling the OnRembServerAvailableBitrate callback.
			if (this.ProcessClusters(nowMs) == BITRATE_UPDATED) {
				updateEstimate = true
			}

		}

		if (this.interArrival.ComputeDeltas(
			timestamp, arrivalTimeMs, nowMs, payloadSize, &tsDelta, &tDelta, &sizeDelta)) {
			var tsDeltaMs float64 = (1000.0 * float64(tsDelta)) / float64(1<<InterArrivalShift)

			this.estimator.Update(tDelta, float64(tsDeltaMs), sizeDelta, this.detector.State(), arrivalTimeMs)
			this.detector.Detect(
				this.estimator.GetOffset(), tsDeltaMs, int(this.estimator.GetNumOfDeltas()), arrivalTimeMs)
		}

		if (!updateEstimate) {
			// Check if it's time for a periodic update or if we should update because
			// of an over-use.
			//mylog.Logger.Infof("55555[%v][%v]\n",this.lastUpdateMs == -1 , nowMs-this.lastUpdateMs > this.remoteRate.GetFeedbackInterval())
			if (this.lastUpdateMs == -1 || nowMs-this.lastUpdateMs > this.remoteRate.GetFeedbackInterval()) {
				updateEstimate = true
			} else if (this.detector.State() == BW_OVERUSING) {
				incomingRate := this.incomingBitrate.GetRate(uint64(arrivalTimeMs))

				if ((incomingRate != 0) && this.remoteRate.TimeToReduceFurther(nowMs, incomingRate)) {
					updateEstimate = true
				}

			}
		}

		if (updateEstimate) {
			// The first overuse should immediately trigger a new estimate.
			// We also have to update the estimate immediately if we are overusing
			// and the target bitrate is too high compared to what we are receiving.
			//mylog.Logger.Infof("6666\n")
			input := RateControlInput{
				this.detector.State(),
				this.incomingBitrate.GetRate(uint64(arrivalTimeMs)),
				this.estimator.GetVarNoise()}

			this.remoteRate.Update(&input, nowMs)
			targetBitrateBps = this.remoteRate.UpdateBandwidthEstimate(nowMs)
			updateEstimate = this.remoteRate.ValidEstimate()
			for k, _ := range this.ssrcs {
				ssrcs.PushBack(k)
			}
		}
	}
	//mylog.Logger.Infof("7777[%v]\n",updateEstimate)
	if (updateEstimate) {
		this.lastUpdateMs = nowMs
		this.AvailableBitrate = targetBitrateBps

		this.listener.OnRembServerAvailableBitrate(this, &ssrcs, targetBitrateBps)
	}

}

func (this *RemoteBitrateEstimatorAbsSendTime) TimeoutStreams(nowMs int64) {
	for k, v := range this.ssrcs {
		if (nowMs-v > streamTimeOutMs) {
			delete(this.ssrcs, k)
		}
	}

	if (0 == len(this.ssrcs)) {
		//mylog.Logger.Infof("TimeoutStreams88888\n")
		interArrival := NewInterArrival(uint32((TimestampGroupLengthMs<<InterArrivalShift)/1000), TimestampToMs, true)
		this.interArrival = &interArrival
		estimator := NewOveruseEstimator(NewOverUseDetectorOptions())
		this.estimator = &estimator
	}
}

func (this *RemoteBitrateEstimatorAbsSendTime) LatestEstimate(ssrcs *deque.Deque, bitrateBps *uint32) bool {
	if (!this.remoteRate.ValidEstimate()) {
		return false
	}

	var ssrcsbak deque.Deque
	for k, _ := range this.ssrcs {
		ssrcsbak.PushBack(k)
	}
	*ssrcs = ssrcsbak

	if (0 == len(this.ssrcs)) {
		*bitrateBps = 0
	} else {
		*bitrateBps = this.remoteRate.LatestEstimate()
	}

	return true
}
