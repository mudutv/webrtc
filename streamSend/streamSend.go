package streamSend

import (
	"github.com/pion/rtcp"
	"github.com/pion/utils"
	"github.com/pion/webrtc/mylog"
	"github.com/pion/webrtc/rateCalculator"
	"github.com/pion/webrtc/rtpstream"
	"github.com/pion/webrtc/uvtime"
	"math"
)

import (
	"github.com/pion/rtp"
)

const MtuSize = 1500
const DeStorageSize = 600

// Don't retransmit packets older than this (ms).
const MaxRetransmissionDelay = 2000

const DefaultRtt  = 100


type Listener interface {
	OnRtpStreamRetransmitRtpPacket(streamSend*StreamSend, packet *rtp.Packet,probation bool)
}

type StorageItem struct {
	// Cloned packet.
	Packet *rtp.Packet
	// Memory to hold the cloned packet (with extra space for RTX encoding).
	//Store [MtuSize + 100]  uint8
	// Last time this packet was resent.
	ResentAtTime uint64
	// Number of times this packet was resent.
	SentTimes uint8
	// Whether the packet has been already RTX encoded.
	RtxEncoded bool

}

type StreamSend struct {
	Buffer     []*StorageItem
	BufferSize int
	Storage    []StorageItem
	BufferStartIdx uint16

	nackCount int
	nackPacketCount int

	rtpstream.RtpStream
	MapSsrc uint32

	Rtt     float64

	transmissionCounter *rateCalculator.RtpDataCounter

	listener Listener

	lostPrior uint32 // Packets lost at last interval.
	sentPrior uint32// Packets sent at last interval.
}

func (this *StreamSend) Close() {
	this.transmissionCounter = nil

}

func NewStreamSend(bufferSize int, params rtpstream.Params,listener Listener) *StreamSend {
	node := StreamSend{}

	mylog.Logger.Infof("new NewStreamSend params %v\n", params)

	if 0 == bufferSize{
		bufferSize = DeStorageSize
	}

	node.Storage = make([]StorageItem, bufferSize, bufferSize)
	for _,v:= range node.Storage{
		v.ResetStorageItem()
	}

	node.Buffer = make([]*StorageItem, 65536,65536)

	node.listener = listener
	node.Params = params
	node.transmissionCounter = rateCalculator.NewRtpDataCounter()

	return &node

}

func (this *StorageItem) ResetStorageItem(){
	this.Packet       = nil;
	this.ResentAtTime = 0;
	this.SentTimes    = 0;
	this.RtxEncoded   = false;
}

func (this *StreamSend) UpdateBufferStartIdx(){
	seq := this.BufferStartIdx +1

	for idx := 0; idx < len(this.Buffer);idx, seq = idx+1, seq+1{
		storageItem := this.Buffer[seq];
		if (nil != storageItem) {
			this.BufferStartIdx = seq;
			break;
		}
	}
}
func (this *StreamSend) StorePacket(packet *rtp.Packet)  {
	updateflag := true

	seq := packet.SequenceNumber
	mylog.Logger.Debugf("store packet %v\n",seq)
	storageItem := this.Buffer[seq]
	if 0 == this.BufferSize{
		storageItem = &this.Storage[0]
		this.Buffer[seq] = storageItem

		this.BufferSize++
		this.BufferStartIdx = seq

	}else if  nil != storageItem{
		mylog.Logger.Infof("store packet11 %v\n",seq)
		storedPacket := storageItem.Packet
		if storedPacket.Timestamp == packet.Timestamp{
			updateflag = false
			return
		}
		mylog.Logger.Infof("store packet22 %v\n",seq)
		storageItem.ResetStorageItem()

		if (this.BufferStartIdx == seq){
			this.UpdateBufferStartIdx()
		}

	}else if (this.BufferSize < len(this.Storage)){
		storageItem = &this.Storage[this.BufferSize]
		this.Buffer[seq] = storageItem
		this.BufferSize++
	}else{
		firstStorageItem := this.Buffer[this.BufferStartIdx]
		firstStorageItem.ResetStorageItem()

		this.Buffer[this.BufferStartIdx] = nil

		this.UpdateBufferStartIdx()

		storageItem = firstStorageItem
		this.Buffer[seq] = storageItem
	}

	//storageItem.Packet = packet
	if true == updateflag{
		storageItem.Packet = packet.Clone()
	}

}

func (this *StreamSend)FillRetransmissionContainer(outSeq []uint16) []*StorageItem{
	var OutItem []*StorageItem = make([]*StorageItem,0,20)

	now := uvtime.GettimeMs()
	var tooOldPacketFound bool = false
	var rtt float64 = DefaultRtt
	if (0 != this.Rtt){
		rtt = this.Rtt
	}

	for _, seq := range outSeq{
		storageItem := this.Buffer[seq]
		var packet *rtp.Packet = nil
		var diffMs  uint32 = 0

		if (nil !=storageItem){
			packet = storageItem.Packet
			diffTs := this.MaxPacketTs - packet.Timestamp
			diffMs = diffTs * 1000 / this.Params.ClockRate
		}

		if (nil == storageItem){
			mylog.Logger.Infof("storageItem nil seq[%v]\n",seq)

		}else if (diffMs > MaxRetransmissionDelay){
			if (!tooOldPacketFound){
				mylog.Logger.Infof("ignoring retransmission for too old packet [seq:%v, max age:%v ms, packet age:%v ms]\n", packet.SequenceNumber, MaxRetransmissionDelay, diffMs)
				tooOldPacketFound = true
			}
		}else if((storageItem.ResentAtTime != 0)&&((uint64(now) - storageItem.ResentAtTime) <= uint64(rtt))){
			mylog.Logger.Infof("ignoring retransmission for a packet already resent in the last RTT ms  [seq:%v, rrt:%v ms]\n", packet.SequenceNumber, uint32(rtt))
		}else {
			storageItem.ResentAtTime = uint64(now)
			storageItem.SentTimes++
			OutItem = append(OutItem, storageItem)
		}
	}

	return OutItem
}

func (this *StreamSend) ReceiveNack(nack *rtcp.TransportLayerNack) {
	this.nackCount++
	for _,item := range nack.Nacks{
		outSeq := item.PacketList()
		mylog.Logger.Infof("StreamSend nack need lost  %v\n", outSeq)
		this.nackPacketCount = this.nackPacketCount + len(outSeq)
		outSend := this.FillRetransmissionContainer(outSeq)
		for _,storageItem := range outSend{
			if (nil == storageItem){
				break
			}

			packet := storageItem.Packet
			mylog.Logger.Infof("StreamSend send nack packet SequenceNumber[%v] ssrc[%v] pt[%v]\n", packet.SequenceNumber,packet.SSRC,packet.PayloadType)
			this.listener.OnRtpStreamRetransmitRtpPacket(this, packet,false)
			this.RtpStream.PacketRetransmitted(packet)
			if (1 == storageItem.SentTimes){
				this.RtpStream.PacketRepaired(packet)
			}

		}
	}
}

func (this *StreamSend) ReceivePacket(packet *rtp.Packet) bool{
	if (!this.RtpStream.ReceivePacket(packet)){
		return false
	}
	if (nil != this.Storage){
		this.StorePacket(packet)
	}
	this.transmissionCounter.Update(packet)
	return true
}

func (this *StreamSend)ReceiveKeyFrameRequestPLI( ){
	this.PliCount++
}

func (this *StreamSend)GetBitrate( now uint64) uint32{
	return this.transmissionCounter.GetBitrate(now)
}

func (this *StreamSend)GetRtcpSenderReport(now uint64) *rtcp.SenderReport{
	if (0 == this.transmissionCounter.GetPacketCount()){
		return nil
	}
	ntp := utils.TimeMs2Ntp(now)
	report := rtcp.SenderReport{}
	diffMs := now - this.MaxPacketMs
	diffTs := diffMs * uint64(this.Params.ClockRate) / 1000

	report.SSRC = this.Params.SSRC
	//report.NTPTime = utils.ToNtpTime(time.Now())

	report.NTPTime |= uint64(ntp.Seconds)
	report.NTPTime = report.NTPTime << 32
	report.NTPTime |= uint64(ntp.Fractions)

	report.RTPTime = uint32(uint64(this.MaxPacketTs) + diffTs)
	report.PacketCount = uint32(this.transmissionCounter.GetPacketCount())
	report.OctetCount = uint32(this.transmissionCounter.GetBytes())

	this.LastSenderReportNtpMs = now
	this.LastSenderReporTs =uint32(uint64(this.MaxPacketTs) + diffTs)

	return &report

}

func (this *StreamSend)GetRtcpSdesChunk()*rtcp.SourceDescription{
	report := rtcp.SourceDescription{}
	chuck := rtcp.SourceDescriptionChunk{Source:this.Params.SSRC,Items:[]rtcp.SourceDescriptionItem{ {rtcp.SDESCNAME,this.Params.Cname}}}
	report.Chunks = append(report.Chunks,chuck)
	return &report
}

func (this *StreamSend)ReceiveRtcpReceiverReport(report rtcp.ReceptionReport){
	 now := uint64(uvtime.GettimeMs())
	 ntp     := utils.TimeMs2Ntp(now)

	// Get the compact NTP representation of the current timestamp.
	var compactNtp uint32 = (ntp.Seconds & 0x0000FFFF) << 16

	compactNtp |= (ntp.Fractions & 0xFFFF0000) >> 16

	var lastSr uint32	  = report.LastSenderReport
	var dlsr uint32   = report.Delay

	// RTT in 1/2^16 second fractions.
	var rtt uint32

	// If no Sender Report was received by the remote endpoint yet, ignore lastSr
	// and dlsr values in the Receiver Report.
	if (0 == lastSr || 0 == dlsr){
		rtt = 0
	} else if (compactNtp > dlsr + lastSr){
		rtt = compactNtp - dlsr - lastSr
	} else{
		rtt = 0
	}


	// RTT in milliseconds.
	this.Rtt = float64((rtt >> 16) * 1000)
	this.Rtt += (float64(rtt & 0x0000FFFF) / float64(65536)) * 1000.0
	mylog.Logger.Infof("this.Rtt[%v] rtt[%v]\n",uint64(this.Rtt), rtt)

	this.PacketsLost  = report.TotalLost
	this.FractionLost = report.FractionLost
	this.UpdateScore(report)
}

func (this *StreamSend)UpdateScore(report rtcp.ReceptionReport){
	// Calculate number of packets sent in this interval.
	totalSent := uint32(this.transmissionCounter.GetPacketCount())
	sent      := uint32(totalSent) - this.sentPrior

	this.sentPrior = totalSent

	// Calculate number of packets lost in this interval.
	var totalLost uint32 = 0
	var lost uint32 = 0
	if ( report.TotalLost > 0){
		totalLost = report.TotalLost
	}

	if (totalLost < this.lostPrior){
		lost = 0
	} else{
		lost = totalLost - this.lostPrior
	}
	this.lostPrior = totalLost

	// Calculate number of packets repaired in this interval.
	totalRepaired := this.PacketsRepaired
	 repaired  := uint32(totalRepaired - this.RepairedPrior)

	this.RepairedPrior = totalRepaired

	// Calculate number of packets retransmitted in this interval.
	totatRetransmitted := this.PacketsRetransmitted
	retransmitted  := totatRetransmitted - this.RetransmittedPrior

	this.RetransmittedPrior = totatRetransmitted

	// We didn't send any packet.
	if (sent == 0) {
	    this.RtpStream.UpdateScore(10)
		return;
	}

	if (lost > sent){
		lost = sent
	}

	if (repaired > lost) {
		//if (HasRtx())
		//{
		//	repaired = lost;
		//	retransmitted -= repaired - lost;
		//}
		//else
		//{
			lost = repaired
		//}
	}

	 repairedRatio  := float64(repaired) / float64(sent)
	 repairedWeight := math.Pow(1.0 / (repairedRatio + 1), 4.0)

	if (retransmitted > 0){
		repairedWeight *= float64(repaired) / float64(retransmitted)
	}

	lost -= repaired* uint32(repairedWeight)

	 deliveredRatio := float64(sent - lost) / float64(sent)
	 score          := uint8(utils.LroundMy(math.Pow(deliveredRatio, 4) * 10))
	 this.RtpStream.UpdateScore(score)
}