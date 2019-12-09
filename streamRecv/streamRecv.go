package streamRecv

import (
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/utils"
	"github.com/pion/webrtc/mylog"
	"github.com/pion/webrtc/nackGenerator"
	"github.com/pion/webrtc/rateCalculator"
	"github.com/pion/webrtc/rtpstream"
	"github.com/pion/webrtc/uvtime"
	"math"
)

type Listener interface {
	OnRtpStreamSendRtcpPacket(streamRecv *StreamRecv, packet []rtcp.Packet)
	OnRtpStreamNeedWorstRemoteFractionLost(rtpStream *StreamRecv, worstRemoteFractionLost *uint8)
}

type TransmissionCounter struct {
	SpatialLayerCounters []([]*rateCalculator.RtpDataCounter)
}

func NewTransmissionCounter(spatialLayers uint8, temporalLayers uint8) TransmissionCounter {
	this := TransmissionCounter{}
	this.SpatialLayerCounters = make([]([]*rateCalculator.RtpDataCounter), spatialLayers, spatialLayers)

	for i := 0; i < len(this.SpatialLayerCounters); i++ {
		this.SpatialLayerCounters[i] = make([]*rateCalculator.RtpDataCounter, temporalLayers, temporalLayers)
		for j := 0; j < len(this.SpatialLayerCounters[i]); j++ {
			this.SpatialLayerCounters[i][j] = rateCalculator.NewRtpDataCounter()
		}
	}

	return this
}

func (p *TransmissionCounter) Update(packet *rtp.Packet) {
	spatialLayer := packet.GetSpatialLayer()
	temporalLayer := packet.GetTemporalLayer()
	if (spatialLayer > uint8(len(p.SpatialLayerCounters)-1)) {
		spatialLayer = uint8(len(p.SpatialLayerCounters) - 1)
	}

	if (temporalLayer > uint8(len(p.SpatialLayerCounters[0])-1)) {
		temporalLayer = uint8(len(p.SpatialLayerCounters[0]) - 1)
	}
	counter := p.SpatialLayerCounters[spatialLayer][temporalLayer]
	counter.Update(packet)
}

func (p *TransmissionCounter) GetBitrate(now uint64) uint32 {
	var rate uint32 = 0
	for i := 0; i < len(p.SpatialLayerCounters); i++ {
		for j := 0; j < len(p.SpatialLayerCounters[i]); j++ {
			rate += p.SpatialLayerCounters[i][j].GetBitrate(now)
		}
	}

	return rate
}

func (p *TransmissionCounter) GetBitrateByLayer(now uint64, spatialLayer uint8, temporalLayer uint8) uint32 {
	var rate uint32 = 0

	if (int(spatialLayer) >= len(p.SpatialLayerCounters)) {
		mylog.Logger.Errorf("spatialLayer too high\n")
		return 0
	}

	if (int(temporalLayer) >= len(p.SpatialLayerCounters[spatialLayer])) {
		mylog.Logger.Errorf("temporalLayer too high\n")
		return 0
	}

	counter := p.SpatialLayerCounters[spatialLayer][temporalLayer]
	if (counter.GetBitrate(now) == 0) {
		return 0
	}

	for i := 0; i <= int(spatialLayer); i++ {
		for j := 0; j <= int(temporalLayer); j++ {
			rate += p.SpatialLayerCounters[i][j].GetBitrate(now)
		}
	}

	return rate
}

func (p *TransmissionCounter) GetSpatialLayerBitrate(now uint64, spatialLayer uint8) uint32 {
	var rate uint32 = 0

	if (int(spatialLayer) >= len(p.SpatialLayerCounters)) {
		mylog.Logger.Errorf("GetSpatialLayerBitrate spatialLayer too high\n")
		return 0
	}

	for i := 0; i < len(p.SpatialLayerCounters[spatialLayer]); i++ {
		rate += p.SpatialLayerCounters[spatialLayer][i].GetBitrate(now)
	}

	return rate
}

func (p *TransmissionCounter) GetLayerBitrate(now uint64, spatialLayer uint8, temporalLayer uint8) uint32 {
	if (int(spatialLayer) >= len(p.SpatialLayerCounters)) {
		mylog.Logger.Errorf("GetLayerBitrate spatialLayer too high\n")
		return 0
	}

	if (int(temporalLayer) >= len(p.SpatialLayerCounters[spatialLayer])) {
		mylog.Logger.Errorf("GetLayerBitrate temporalLayer too high\n")
		return 0
	}

	return p.SpatialLayerCounters[spatialLayer][temporalLayer].GetBitrate(now)
}

func (p *TransmissionCounter) GetPacketCount() int {
	var packetCount int = 0
	for i := 0; i < len(p.SpatialLayerCounters); i++ {
		for j := 0; j < len(p.SpatialLayerCounters[i]); j++ {
			packetCount += p.SpatialLayerCounters[i][j].GetPacketCount()
		}
	}

	return packetCount
}

func (p *TransmissionCounter) GetBytes() uint64 {
	var bytes uint64 = 0
	for i := 0; i < len(p.SpatialLayerCounters); i++ {
		for j := 0; j < len(p.SpatialLayerCounters[i]); j++ {
			bytes += p.SpatialLayerCounters[i][j].GetBytes()
		}
	}

	return bytes
}

type StreamRecv struct {
	NackGeneratorNode *nackGenerator.NackGenerator

	rtpstream.RtpStream

	NackCount       int
	NackPacketCount int
	listener        Listener

	Transit             uint32 //  Relative transit time for prev packet.
	Jitter              uint32
	transmissionCounter TransmissionCounter
	LastPacketAt        uint64

	ReceivedPrior      uint32 // Packets received at last interval
	ReportedPacketLost uint32 //一个rr周期内，丢失的包的个数

	LastSrTimestamp uint32 // The middle 32 bits out of 64 in the NTP // timestamp received in the most recent // sender report.
	LastSrReceived  uint64 // Wallclock time representing the most recent sender report arrival.
}

func (this *StreamRecv) Close() {
	if (nil != this.NackGeneratorNode) {
		this.NackGeneratorNode.Close()
	}

	this.NackGeneratorNode = nil

}

func NewStreamRecv(params rtpstream.Params, listener Listener) *StreamRecv {
	node := StreamRecv{}

	mylog.Logger.Infof("new StreamRecv params %v\n", params)

	if (true == params.UseNack) {
		node.NackGeneratorNode = nackGenerator.NewNackGenerator(&node)
	}

	node.Params = params
	node.listener = listener
	node.Score = 10
	node.transmissionCounter = NewTransmissionCounter(params.SpatialLayers, params.TemporalLayers)

	return &node

}

func (this *StreamRecv) ReceivePacket(packet *rtp.Packet) bool {
	if (!this.RtpStream.ReceivePacket(packet)) {
		return false
	}

	// Process the packet at codec level.
	if (packet.PayloadType == this.Params.PayloadType) {
		//rtp.ProcessRtpPacket(packet, rtp.CodeType{rtp.MIME_TYPE_VIDEO, rtp.SUB_TYPE_VP8})
		rtp.ProcessRtpPacket(packet, this.Params.MimeType)
	}

	if (this.Params.UseNack == true) {
		if (true == this.NackGeneratorNode.ReceivePacket(packet)) {
			this.RtpStream.PacketRetransmitted(packet)
			this.RtpStream.PacketRepaired(packet)
		}

	}

	this.CalculateJitter(packet.Timestamp)
	this.transmissionCounter.Update(packet)
	//if (packet.SequenceNumber%500 == 0) {
	//	mylog.Logger.Infof("add remb rev GetBytes[%v] GetPacketCount[%v] GetBitrate[%v]\n", this.transmissionCounter.GetBytes(), this.transmissionCounter.GetPacketCount(), this.transmissionCounter.GetBitrate(uint64(uvtime.GettimeMs())))
	//}

	this.LastPacketAt = uint64(uvtime.GettimeMs())
	return true

}

func (this *StreamRecv) ReceiveRtxPacket(packet *rtp.Packet) bool {
	if (!this.Params.UseNack) {
		mylog.Logger.Infof("NACK not supported")
		return false
	}
	// Check that the payload type corresponds to the one negotiated.
	if (packet.PayloadType != this.Params.RtxPayloadType) {
		mylog.Logger.Infof("ignoring RTX packet with invalid payload type [ssrc:%v, seq:%v, pt:%v]",
			packet.SSRC,
			packet.SequenceNumber,
			packet.PayloadType)

		return false
	}

	rtxSeq := packet.SequenceNumber
	if (!packet.RtxDecode(this.Params.PayloadType, this.Params.SSRC)) {
		mylog.Logger.Errorf(
			"ignoring empty RTX packet [ssrc:%v, seq:%v, pt:%v]",
			packet.SSRC,
			packet.SequenceNumber,
			packet.PayloadType)

		return false;
	}

	mylog.Logger.Infof(
		"received RTX packet [ssrc:%v, seq:%v] recovering original [ssrc:%v, seq:%v]",
		this.Params.RtxSsrc,
		rtxSeq,
		packet.SSRC,
		packet.SequenceNumber)

	if (!this.UpdateSeq(packet)) {
		mylog.Logger.Errorf("invalid RTX packet [ssrc:%v, seq:%v]",
			packet.SSRC,
			packet.SequenceNumber)
		return false;
	}
	// Process the packet at codec level.
	if (packet.PayloadType == this.Params.PayloadType) {
		//rtp.ProcessRtpPacket(packet, rtp.CodeType{rtp.MIME_TYPE_VIDEO, rtp.SUB_TYPE_VP8})
		rtp.ProcessRtpPacket(packet, this.Params.MimeType)
	}

	this.PacketRetransmitted(packet)

	if (this.NackGeneratorNode.ReceivePacket(packet)) {
		// Mark the packet as repaired.
		this.PacketRepaired(packet)

		// Increase transmission counter.
		this.transmissionCounter.Update(packet)

		// Update last packet arrival.
		this.LastPacketAt = uint64(uvtime.GettimeMs())

		return true;
	}

	return false;
}

//lister NackGenerator
func (this *StreamRecv) OnNackGeneratorNackRequired(nackBatch []uint16) {
	var p *uint16 = nil
	var bitmask uint16 = 0
	var NumSend int = 0

	//fmt.Println("begin OnNackGeneratorNackRequired ")
	NackPacket := rtcp.TransportLayerNack{0, this.Params.SSRC, []rtcp.NackPair{}}
	mylog.Logger.Infof("StreamRecv send nack total packetId[%v] ssrc[%v]\n", nackBatch, this.Params.SSRC)
	for index, v := range nackBatch {
		NumSend++
		if nil == p {
			p = &nackBatch[index]
			continue
		}

		shift := v - *p - 1
		if shift <= 15 {
			bitmask |= (1 << shift)
			continue
		}

		//mylog.Logger.Infof("StreamRecv send nack packetId[%v]   bitmask[%x] ssrc[%v]\n", *p, bitmask, this.Params.SSRC)
		NackPacket.Nacks = append(NackPacket.Nacks, rtcp.NackPair{*p, rtcp.PacketBitmap(bitmask)})
		p = &nackBatch[index]
		bitmask = 0

	}

	if nil != p {
		//mylog.Logger.Infof("StreamRecv send nack packetId[%v]   bitmask[%x] ssrc[%v]\n", *p, bitmask, this.Params.SSRC)
		NackPacket.Nacks = append(NackPacket.Nacks, rtcp.NackPair{*p, rtcp.PacketBitmap(bitmask)})
	}
	//stream := this.GetStreamRecv(ssrc)
	this.NackCount++
	this.NackPacketCount = this.NackPacketCount + NumSend
	mylog.Logger.Infof("StreamRecv send nack [%s]", NackPacket.String())
	this.listener.OnRtpStreamSendRtcpPacket(this, []rtcp.Packet{&NackPacket})

	//if rtcpSendErr := this.PeerConnection.WriteRTCP([]rtcp.Packet{&NackPacket}); rtcpSendErr != nil {
	//	mylog.Logger.Errorf("producer OnNackGeneratorNackRequired  write rtcp fail[%s]\n",rtcpSendErr.Error())
	//}

	//fmt.Println("end OnNackGeneratorNackRequired")

}

//lister NackGenerator
func (this *StreamRecv) OnNackGeneratorKeyFrameRequired(ssrc uint32) {
	//if rtcpSendErr := this.PeerConnection.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: ssrc}}); rtcpSendErr != nil {
	//	mylog.Logger.Errorf("producer OnNackGeneratorKeyFrameRequired  write rtcp fail[%s]\n",rtcpSendErr.Error())
	//}

	//this.listener.OnRtpStreamSendRtcpPacket(this,[]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: ssrc}})
	mylog.Logger.Infof("OnNackGeneratorKeyFrameRequired send pli  ssrc[%v]", ssrc)
	this.RequestKeyFrame()
	return
}

func (this *StreamRecv) RequestKeyFrame() {
	if (nil != this.NackGeneratorNode) {
		this.NackGeneratorNode.Reset()
	}

	this.PliCount++
	this.listener.OnRtpStreamSendRtcpPacket(this, []rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: this.Params.SSRC}})
}

func (this *StreamRecv) CalculateJitter(rtpTimestamp uint32) {

	//设接收到两个rtp包的时间间隔，换算成以sample为单位。(Rj - Ri)
	//
	//两个rtp包中rtp时间戳间隔为：(Sj - Si)
	//
	//则该次jitter为D(i,j) = (Rj - Ri) - (Sj - Si)
	//
	//总的jitter值取平均，计算公式为：J(i) = J(i-1) + (|D(i-1,i)| - J(i-1))/16
	//
	// 
	//
	//rtcp中jitter值以sample为单位，换算成ms的公式为：jitter*1000／samplerate
	//---------------------
	//	版权声明：本文为CSDN博主「lipku」的原创文章，遵循CC 4.0 by-sa版权协议，转载请附上原文出处链接及本声明。
	//原文链接：https://blog.csdn.net/lipku/article/details/50183405
	if 0 == this.Params.ClockRate {
		mylog.Logger.Errorf("CalculateJitter clock is 0  ssrc[%v]", this.Params.SSRC)
		return
	}
	transit := int(uint32(uvtime.GettimeMs()) - (rtpTimestamp * 1000 / this.Params.ClockRate))
	d := transit - int(this.Transit)

	this.Transit = uint32(transit)
	if (d < 0) {
		d = -d
	}
	this.Jitter += uint32((1. / 16.) * (float64(d) - float64(this.Jitter)))
}

func (this *StreamRecv) GetRtcpReceiverReport() rtcp.ReceptionReport {
	worstRemoteFractionLost := uint8(0)
	if (this.Params.UseInBandFec) {
		this.listener.OnRtpStreamNeedWorstRemoteFractionLost(this, &worstRemoteFractionLost)

		if (worstRemoteFractionLost > 0) {
			mylog.Logger.Infof("using worst remote fraction lost:%d", worstRemoteFractionLost)
		}
	}

	rr := rtcp.ReceptionReport{}
	rr.SSRC = this.Params.SSRC

	prevPacketsLost := this.PacketsLost
	expected := this.GetExpectedPackets()
	this.PacketsLost = expected - uint32(this.transmissionCounter.GetPacketCount())

	expectedInterval := expected - this.ExpectedPrior
	this.ExpectedPrior = expected

	receivedInterval := uint32(this.transmissionCounter.GetPacketCount()) - this.ReceivedPrior
	this.ReceivedPrior = uint32(this.transmissionCounter.GetPacketCount())

	//期待收到的包-实际收到的包（上一次到现在），算出一个周期里丢失的包个数
	lostInterval := expectedInterval - receivedInterval
	if (expectedInterval == 0 || lostInterval <= 0) {
		this.FractionLost = 0
	} else {
		this.FractionLost = uint8((lostInterval << 8) / expectedInterval)
	}

	if (worstRemoteFractionLost <= this.FractionLost) {
		this.ReportedPacketLost += (this.PacketsLost - prevPacketsLost)
		//rr.TotalLost = this.ReportedPacketLost
		rr.SetTotalLost(int32(this.ReportedPacketLost))
		rr.FractionLost = this.FractionLost
	} else {
		// Recalculate packetsLost.
		var newLostInterval uint32 = (uint32(worstRemoteFractionLost) * expectedInterval) >> 8
		var newReceivedInterval uint32 = expectedInterval - newLostInterval
		this.ReportedPacketLost += (receivedInterval - newReceivedInterval)
		//rr.TotalLost = this.ReportedPacketLost
		rr.SetTotalLost(int32(this.ReportedPacketLost))
		rr.FractionLost = worstRemoteFractionLost
	}
	rr.LastSequenceNumber = uint32(this.MaxSeq) + this.Cycles
	rr.Jitter = this.Jitter

	if (this.LastSrReceived != 0) {
		// Get delay in milliseconds.
		var delayMs uint32 = uint32(uint64(uvtime.GettimeMs()) - this.LastSrReceived)
		// Express delay in units of 1/65536 seconds.
		var dlsr uint32 = (delayMs / 1000) << 16
		dlsr |= uint32((delayMs % 1000) * 65536 / 1000)

		rr.Delay = dlsr
		rr.LastSenderReport = this.LastSrTimestamp
	} else {
		rr.Delay = 0
		rr.LastSenderReport = 0
	}

	mylog.Logger.Infof("add rr TotalLost[%v] SSRC[%x] FractionLost[%v] lostInterval[%v] expectedInterval[%v] LastSequenceNumber[%v] Jitter[%v]  Delay[%v] LastSenderReport[%v]", rr.TotalLost, rr.SSRC, rr.FractionLost, lostInterval, expectedInterval, rr.LastSequenceNumber, rr.Jitter, rr.Delay, rr.LastSenderReport)
	return rr

}

func (this *StreamRecv) ReceiveRtcpSenderReport(sr *rtcp.SenderReport) {
	this.LastSrReceived = uint64(uvtime.GettimeMs())
	this.LastSrTimestamp = uint32(((sr.NTPTime >> 32) & 0xffffffff) << 16)
	this.LastSrTimestamp += uint32(sr.NTPTime & 0xffffffff >> 16)

	// Update info about last Sender Report.
	// NOLINT(cppcoreguidelines-pro-type-member-init)
	ntp := utils.Ntp{}

	ntp.Seconds = uint32(((sr.NTPTime >> 32) & 0xffffffff))
	ntp.Fractions = uint32(sr.NTPTime & 0xffffffff)

	this.LastSenderReportNtpMs = utils.Ntp2TimeMs(ntp)
	this.LastSenderReporTs = sr.RTPTime

	// Update the score with the current RR.
	this.UpdateScore()
}

func (this *StreamRecv) UpdateScore() {
	// Calculate number of packets expected in this interval.
	totalExpected := this.GetExpectedPackets()
	expected := totalExpected - this.ExpectedPrior

	this.ExpectedPrior = totalExpected

	// Calculate number of packets received in this interval.
	totalReceived := this.transmissionCounter.GetPacketCount()
	received := uint32(totalReceived) - this.ReceivedPrior

	this.ReceivedPrior = uint32(totalReceived)

	// Calculate number of packets lost in this interval.
	lost := uint32(0)

	if (expected < received) {
		lost = 0
	} else {
		lost = expected - received
	}

	// Calculate number of packets repaired in this interval.
	totalRepaired := this.PacketsRepaired
	repaired := totalRepaired - this.RepairedPrior

	this.RepairedPrior = totalRepaired

	// Calculate number of packets retransmitted in this interval.
	totatRetransmitted := this.PacketsRetransmitted
	retransmitted := totatRetransmitted - this.RetransmittedPrior

	this.RetransmittedPrior = totatRetransmitted

	//if (this.Inactive){
	//	return;
	//}

	// We didn't expect more packets to come.
	if (expected == 0) {
		this.RtpStream.UpdateScore(10)

		return;
	}

	if (lost > received) {
		lost = received
	}

	if (uint32(repaired) > lost) {
		//if (HasRtx())
		//{
		//	repaired = lost;
		//	retransmitted -= repaired - lost;
		//}
		//else
		//{
		lost = uint32(repaired)
		//}
	}

	repairedRatio := float64(repaired) / float64(received)
	repairedWeight := math.Pow(1.0/(repairedRatio+1), 4.0)

	if (retransmitted > 0) {
		repairedWeight *= float64(repaired) / float64(retransmitted)
	}

	lost -= uint32(repaired) * uint32(repairedWeight)

	deliveredRatio := float64(received-lost) / float64(received)
	score := uint8(utils.LroundMy(math.Pow(deliveredRatio, 4) * 10))
	this.RtpStream.UpdateScore(score)

}
