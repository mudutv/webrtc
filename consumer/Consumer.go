package consumer

import (
	"github.com/pion/webrtc"
	"github.com/pion/webrtc/notedit/sdp"
	"github.com/pion/webrtc/rtpHeaderExtensionIds"
)

const   (
	ConsumerType_NONE = iota
	ConsumerType_SIMPLE
	ConsumerType_SIMULCAST
	ConsumerType_SVC
	ConsumerType_PIPE
	)


//type ListenerChild interface {
//	SendRtpPacket(packet *rtp.Packet)
//	GetTransmissionRate(now uint64) uint32
//	ReceiveNack(nack *rtcp.TransportLayerNack)
//	ReceiveKeyFrameRequestPLI(packet *rtcp.PictureLossIndication)
//    GetRtcp(packet *compoundrtcp.CompoundRtcp, rtpStream *streamSend.StreamSend,now uint64)
//	ProducerRtpStream(stream *streamRecv.StreamRecv)
//	GetRtpStreams() map[uint32]*streamSend.StreamSend
//	ReceiveRtcpReceiverReport(report rtcp.ReceptionReport)
//	ProducerRtcpSenderReport(rtpStream *streamRecv.StreamRecv, first bool)
//	NeedWorstRemoteFractionLost(mappedSsrc uint32, worstRemoteFractionLost *uint8)
//	GetWriteTrack()*webrtc.Track
//	Close()
//}

type Consumer struct {
	Id string
	Kind  webrtc.RTPCodecType
	ConsumerType int


	RtpHeaderExtensionIds  rtpHeaderExtensionIds.RtpHeaderExtensionIds

	mediaSsrcs []uint32

	MaxRtcpInterval uint16
	LastRtcpSentTime  uint64


	//listenerChild ListenerChild



	Info *sdp.SDPInfo
	Track *sdp.TrackInfo
}

func (node *Consumer)NewConsumer(track *sdp.TrackInfo, info *sdp.SDPInfo, id string,  consumerType int) {
	node.Id =id
	node.ConsumerType = consumerType
	node.Info = info
	node.Track = track
	media := track.GetMedia()
	if "video" == media {
		node.Kind =  webrtc.RTPCodecTypeVideo
		node.MaxRtcpInterval = rtpHeaderExtensionIds.MaxVideoIntervalMs
	} else{
		node.Kind =  webrtc.RTPCodecTypeAudio
		node.MaxRtcpInterval = rtpHeaderExtensionIds.MaxAudioIntervalMs
	}
	mediainfo := info.GetMedia(media)
	ext := mediainfo.GetExtensions()
	node.RtpHeaderExtensionIds.InitRtpHeaderExtensionIds(ext)

	node.mediaSsrcs = make([]uint32,0,3)
	for _,ssrc := range track.GetSSRCS(){
		node.mediaSsrcs = append(node.mediaSsrcs, uint32(ssrc))
	}

	return
}

//func (this *Consumer)Close(){
//	this.Info = nil
//	this.Track = nil
//	this.listenerChild.Close()
//}



//func (this *Consumer)ProducerRtpStream(stream *streamRecv.StreamRecv){
//	this.listenerChild.ProducerRtpStream(stream)
//}

func (this *Consumer)GetMediaSsrcs() []uint32 {
	return this.mediaSsrcs
}

//func (this *Consumer)SendRtpPacket(packet *rtp.Packet){
//	this.listenerChild.SendRtpPacket(packet)
//}

//func (this *Consumer)GetTransmissionRate(now uint64) uint32{
//	return this.listenerChild.GetTransmissionRate(now)
//}

//func (this *Consumer)ReceiveNack(nack *rtcp.TransportLayerNack){
//	this.listenerChild.ReceiveNack(nack)
//}

//func (this *Consumer)ReceiveKeyFrameRequestPLI(packet *rtcp.PictureLossIndication){
//	this.listenerChild.ReceiveKeyFrameRequestPLI(packet)
//}

//func (this *Consumer)GetRtcp(packet *compoundrtcp.CompoundRtcp, rtpStream *streamSend.StreamSend,now uint64){
//	this.listenerChild.GetRtcp(packet,rtpStream,now)
//}
//
//func (this *Consumer)GetRtpStreams() map[uint32]*streamSend.StreamSend{
//	return this.listenerChild.GetRtpStreams()
//}
//
//func (this *Consumer)ReceiveRtcpReceiverReport(report rtcp.ReceptionReport){
//	this.listenerChild.ReceiveRtcpReceiverReport(report)
//}
//
//func (this *Consumer)ProducerRtcpSenderReport(rtpStream *streamRecv.StreamRecv, first bool){
//	this.listenerChild.ProducerRtcpSenderReport(rtpStream,first)
//}
//
//func (this *Consumer)NeedWorstRemoteFractionLost(mappedSsrc uint32, worstRemoteFractionLost *uint8){
//	this.listenerChild.NeedWorstRemoteFractionLost(mappedSsrc,worstRemoteFractionLost)
//}

//func (this *Consumer)GetWriteTrack()*webrtc.Track{
//	return this.listenerChild.GetWriteTrack()
//}

