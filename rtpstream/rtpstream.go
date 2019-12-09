package rtpstream

import (
	"github.com/pion/rtp"
	"github.com/pion/webrtc/mylog"
	"github.com/pion/webrtc/seqManager"
	"github.com/pion/webrtc/uvtime"
)

const RtpSeqMod = (1 << 16)
const  MaxDropout = 3000
const MaxMisorder = 1500


type Params struct {
	SSRC      uint32
	ClockRate uint32
	PayloadType uint8
	RtxPayloadType uint8
	RtxSsrc   uint32

	MimeType  rtp.CodeType
	Rid       string
	Cname     string
	SpatialLayers uint8
	TemporalLayers uint8

	UseNack bool //key:[rtcp-fb], value[124 nack]
	UseFir bool //key:[rtcp-fb], value[124 ccm fir]//ccm是codec control using RTCP feedback message简称，意思是支持使用rtcp反馈机制来实现编码控制，fir是Full Intra Request 简称，意思是接收方通知发送方发送幅完全帧过来
	UsePli bool //key:[rtcp-fb], value[124 nack pli]
	//UseRemb bool

	UseInBandFec bool
	UseDtx bool

}
type RtpStream struct {
	Started bool   	// Whether at least a RTP packet has been received.

	PacketsDiscarded int64
	BaseSeq uint32  // Base seq number.开始收到的第一个rtp的seq
	BadSeq  uint32    // Last 'bad' seq number + 1.
	Cycles uint32

	MaxSeq  uint16     // Highest seq. number seen.
	MaxPacketTs uint32 // Highest timestamp seen.
	MaxPacketMs uint64  // When the packet with highest timestammp was seen.

	Params  Params
	PacketsRetransmitted int64
	RetransmittedPrior int64      // Packets retransmitted at last interval.
	PacketsRepaired       int64
	RepairedPrior int64         // Packets repaired at last interval.

	PliCount    int

	Score       uint8


	LastSenderReportNtpMs uint64 // NTP timestamp in last Sender Report (in ms).
	LastSenderReporTs uint32 // RTP timestamp in last Sender Report.
	PacketsLost uint32    //开始到现在总共丢的包的个数
	ExpectedPrior uint32   //Packets expected at last interval
	FractionLost uint8 //rr包里的丢包率




}


func (this *RtpStream)InitSeq(seq uint16) {
	// Initialize/reset RTP counters.
	this.BaseSeq = uint32(seq)
	this.MaxSeq = seq
	this.BadSeq = RtpSeqMod +1 // So seq == badSeq is false.
}

func (this *RtpStream)UpdateSeq(packet *rtp.Packet) bool{
	seq := packet.SequenceNumber
	udelta := seq - this.MaxSeq

	if (udelta < MaxDropout){
		if (seq < this.MaxSeq){
			this.Cycles += RtpSeqMod
		}

		this.MaxSeq = seq
	}else if(udelta <= RtpSeqMod - MaxMisorder){
		if (uint32(seq) == this.BadSeq){
			mylog.Logger.Infof("too bad sequence number, re-syncing RTP [ssrc:%lu, seq:%u]\n", packet.SSRC,packet.SequenceNumber)
			this.InitSeq(seq)
			this.MaxPacketTs = packet.Timestamp
			this.MaxPacketMs = uint64(uvtime.GettimeMs())
		}else{

			mylog.Logger.Infof("bad sequence number, ignoring packet [ssrc:%lu, seq:%u]\n", packet.SSRC,packet.SequenceNumber)
			this.BadSeq = uint32((seq + 1) & (RtpSeqMod - 1))
			this.PacketsDiscarded++

			return false
		}
	}else {

	}

	return true
}

func (this *RtpStream)ReceivePacket(packet *rtp.Packet) bool{
	seq := packet.SequenceNumber

	if (!this.Started){
		this.InitSeq(seq)

		this.Started = true
		this.MaxSeq = seq -1
		this.MaxPacketTs = packet.Timestamp
		this.MaxPacketMs = uint64(uvtime.GettimeMs())
	}

	if (! this.UpdateSeq(packet)){
		mylog.Logger.Infof("invalid packet [ssrc:%lu, seq:%u]\n", packet.SSRC,packet.SequenceNumber)

		return false
	}

	if (seqManager.CompareTimeStampHigherThan(packet.Timestamp, this.MaxPacketTs)){
		this.MaxPacketTs = packet.Timestamp
		this.MaxPacketMs = uint64(uvtime.GettimeMs())
	}

	return true
}

func (this *RtpStream)PacketRetransmitted(packet *rtp.Packet){
	this.PacketsRetransmitted++
}

func (this *RtpStream)PacketRepaired(packet *rtp.Packet){
	this.PacketsRepaired++
}


//获取预期理想中应该收到的包的个数
func (this *RtpStream)GetExpectedPackets() uint32{
	return (this.Cycles + uint32(this.MaxSeq)) - this.BaseSeq + 1;
}

func (this *RtpStream)GetSenderReportNtpMs() uint64{
	return this.LastSenderReportNtpMs
}

func (this *RtpStream)GetSsrc() uint32{
	return this.Params.SSRC
}

func (this *RtpStream)GetRtxSsrc() uint32{
	return this.Params.RtxSsrc
}

func (this *RtpStream)UpdateScore(score uint8){

}