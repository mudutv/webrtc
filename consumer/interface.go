package consumer
import (
	"github.com/pion/webrtc/streamSend"
	"github.com/pion/webrtc/compoundrtcp"
	"github.com/pion/webrtc/streamRecv"
	"github.com/pion/rtp"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc"
	"github.com/pion/webrtc/notedit/sdp"
)
type InferfaceConsumer interface {
	GetTransmissionRate(now uint64) uint32
	GetRtpStreams() map[uint32]*streamSend.StreamSend
	GetRtcp(packet *compoundrtcp.CompoundRtcp, rtpStream *streamSend.StreamSend,now uint64)
	ID() string
	Close()
	ProducerRtpStream(stream *streamRecv.StreamRecv)
	SendRtpPacket(packet *rtp.Packet)
	SendRtpPacketReadyDoing(packet *rtp.Packet)
	ProducerRtcpSenderReport(rtpStream *streamRecv.StreamRecv, first bool)
	NeedWorstRemoteFractionLost(mappedSsrc uint32, worstRemoteFractionLost *uint8)
	ProducerClosed()
	GetMediaSsrcs() []uint32
	ReceiveRtcpReceiverReport(report rtcp.ReceptionReport)
	ReceiveNack(nack *rtcp.TransportLayerNack)
	ReceiveKeyFrameRequestPLI(packet *rtcp.PictureLossIndication)
    GetWriteTrack(pt uint8)(*webrtc.Track)
	SetMapPt(producerInfo *sdp.SDPInfo,codeName string,pt uint8) bool
	SetMapSSRC(producerTrack *sdp.TrackInfo,ssrc uint32) bool
	AddwriteTrack(pt uint8, writeRtpTrack *webrtc.Track) bool

}
