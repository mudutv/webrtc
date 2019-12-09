package consumer

import (
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/webrtc"
	"github.com/pion/webrtc/compoundrtcp"
	"github.com/pion/webrtc/mylog"
	"github.com/pion/webrtc/notedit/sdp"
	"github.com/pion/webrtc/rtpstream"
	"github.com/pion/webrtc/streamRecv"
	"github.com/pion/webrtc/streamSend"
)
type SimpleConsumer struct {
	Consumer
	keyFrameSupported bool
	rtpStream *streamSend.StreamSend
	SendStreamMap map[uint32]*streamSend.StreamSend

	//writeRtpTrack      *webrtc.Track
	mapWriteRtpTrack  map[uint8]*webrtc.Track
	listener Listener

	mapPt map[uint8]uint8
    mapSSRC map[uint32][]uint
	//chanRecvRtp chan *rtp.Packet


	//producerRtpStream *streamRecv.StreamRecv


}

type Listener interface {
	OnConsumerSendRtpPacket(consumer InferfaceConsumer,packet *rtp.Packet)
	OnConsumerRetransmitRtpPacket(consumer InferfaceConsumer,packet *rtp.Packet,probation bool)
	OnConsumerKeyFrameRequested(consumer InferfaceConsumer,mappedSsrc uint32)
	OnConsumerProducerClosed(consumer InferfaceConsumer)
	OnConsumerRecvRtpPacket(packet *rtp.Packet)
}

func NewSimpleConsumer(track *sdp.TrackInfo, info *sdp.SDPInfo, listener Listener, id string) *SimpleConsumer{
	track.GetEncodings()

	node := SimpleConsumer{}
	node.listener = listener
	node.NewConsumer(track, info,id,  ConsumerType_SIMPLE)
	node.SendStreamMap = make(map[uint32]*streamSend.StreamSend)
	node.mapWriteRtpTrack =make(map[uint8]*webrtc.Track)
	node.mapPt =make(map[uint8]uint8)      //producerpt  consumerpt
	node.mapSSRC =make(map[uint32][]uint)   //consumerssrc  producerssrc
	//node.chanRecvRtp = make(chan *rtp.Packet,300)



	return &node
}

func (this *SimpleConsumer)SetMapSSRC(producerTrack *sdp.TrackInfo,ssrc uint32) bool{
	if len(producerTrack.GetSSRCS()) > 0{
		this.mapSSRC[ssrc] = producerTrack.GetSSRCS()
	}
	return true
}

func  (this *SimpleConsumer)SetMapPt(producerInfo *sdp.SDPInfo,codeName string,pt uint8) bool{
	//G722 = "G722"
	//Opus = "opus"
	//VP8  = "VP8"
	//VP9  = "VP9"
	//H264 = "H264"
	//RED = "red"
	//ULPFEC = "ulpfec"
	//RTX = "rtx"
	var num int = 0
	if ("VP8" == codeName || "H264" == codeName){
		MediaInfo := producerInfo.GetMedia("video")
		if (nil == MediaInfo){
			mylog.Logger.Errorf("SimpleConsumer [%s] codeName[%s] pt[%d] get infoMedia fail", this.Id,codeName,pt)
			return false
		}
		codecs := MediaInfo.GetCodecs()
		for _,codec := range  codecs{
			if codec.GetCodec() == codeName{
				this.mapPt[uint8(codec.GetType())] = pt
				num++
				mylog.Logger.Infof("SimpleConsumer [%s] codeName[%s] pt[%d] add ssrcPt[%d]", this.Id,codeName,pt,uint8(codec.GetType()))
			}
		}
	}

	if ("opus" == codeName){
		MediaInfo := producerInfo.GetMedia("audio")
		if (nil == MediaInfo){
			mylog.Logger.Errorf("SimpleConsumer [%s] codeName[%s] pt[%d] get infoMedia fail", this.Id,codeName,pt)
			return false
		}
		codecs := MediaInfo.GetCodecs()
		for _,codec := range  codecs{
			if codec.GetCodec() == codeName{
				this.mapPt[uint8(codec.GetType())] = pt
				num++
				mylog.Logger.Infof("SimpleConsumer [%s] codeName[%s] pt[%d] add ssrcPt[%d]", this.Id,codeName,pt,uint8(codec.GetType()))
			}
		}
	}

	if num == 0{
		mylog.Logger.Errorf("SimpleConsumer [%s] codeName[%s] pt[%d] len 0", this.Id,codeName,pt)
		return false
	}

	return true
}

func  (this *SimpleConsumer)AddwriteTrack(pt uint8, writeRtpTrack *webrtc.Track) bool{
	this.mapWriteRtpTrack[pt] = writeRtpTrack
	return true
}

func (this *SimpleConsumer) SendRtpPacketReadyDoing(packet *rtp.Packet) {

	streamSend := this.GetStreamSend(packet, packet.SSRC, packet.PayloadType)
	if (nil == streamSend) {
		mylog.Logger.Errorf("miaobinwei111 cant GetStreamSend StreamSend ssrc[%v] PayloadType[%d]", packet.SSRC, packet.PayloadType)
		return
	}

	//mylog.Logger.Infof("miaobinwei111 SendRtpPacket pt[%v] ssrc[%v] origSsrc[%v] seq[%v]", this.writeRtpTrack.PayloadType(),this.writeRtpTrack.SSRC(),origSsrc,packet.SequenceNumber)
	if true == streamSend.ReceivePacket(packet) {
		this.listener.OnConsumerSendRtpPacket(this, packet)
	}

	return
}
func (this *SimpleConsumer) SendRtpPacket(packet *rtp.Packet) {
	if (len(this.mapPt) == 0) {
		mylog.Logger.Errorf("SimpleConsumer id[%s] kind[%s] mapPt 0", this.Id, this.Kind)
		return
	}

	dstpt, ok := this.mapPt[packet.PayloadType]
	if false == ok {
		mylog.Logger.Errorf("SimpleConsumer id[%s] kind[%s] mapPt get srcpt[%d] fail", this.Id, this.Kind, packet.PayloadType)
		return
	}

	writeRtpTrack, ok := this.mapWriteRtpTrack[dstpt]
	if false == ok {
		mylog.Logger.Errorf("SimpleConsumer id[%s] kind[%s] mapWriteRtpTrack get srcpt[%d] dstpt[%d] fail", this.Id, this.Kind, packet.PayloadType, dstpt)
		return
	}

	pk := packet.Clone()
	pk.SSRC = writeRtpTrack.SSRC()
	pk.PayloadType = writeRtpTrack.PayloadType()
	this.listener.OnConsumerRecvRtpPacket(pk)

	return
}

func (this *SimpleConsumer)GetStreamSend(packet *rtp.Packet, ssrc uint32, pt uint8)*streamSend.StreamSend{
	//ssrc := packet.SSRC
	v,ok := this.SendStreamMap[ssrc]
	if ok{
		return v
	}

	mediaInfo := this.Info.GetMedia(this.Track.GetMedia())
	if (nil == mediaInfo){
		return nil
	}

	codecInfo := mediaInfo.GetCodecForType(int(pt))
	if (nil == codecInfo){
		mylog.Logger.Errorf("Consumer GetCodecForType fail  pt[%d]", pt)
		return nil
	}
	fbInfo := codecInfo.GetRTCPFeedbacks()

	params := rtpstream.Params{}
	params.SSRC = ssrc
	if (codecInfo.GetCodec() == "VP8"){
		params.ClockRate = uint32(codecInfo.GetRate())
		params.MimeType.MimeType = rtp.MIME_TYPE_VIDEO
		params.MimeType.SubType = rtp.SUB_TYPE_VP8
	}else if (codecInfo.GetCodec() == "H264"){
		params.ClockRate = uint32(codecInfo.GetRate())
		params.MimeType.MimeType = rtp.MIME_TYPE_VIDEO
		params.MimeType.SubType = rtp.SUB_TYPE_H264
	}else if (codecInfo.GetCodec() == "opus"){
		params.ClockRate = uint32(codecInfo.GetRate())
		params.MimeType.MimeType = rtp.MIME_TYPE_VIDEO
		params.MimeType.SubType = rtp.SUB_TYPE_OPUS
	}else if(codecInfo.GetCodec() == "red" || (codecInfo.GetCodec() == "ulpfec")){
		var pre *sdp.CodecInfo

		for  _,v := range mediaInfo.GetCodecs(){

			if (int(pt) == v.GetType()){
				break
			}
			pre = v

		}

		if nil != pre{
			params.ClockRate = uint32(codecInfo.GetRate())
			fbInfo = pre.GetRTCPFeedbacks()
			if ((pre.GetCodec() == "VP8") || (pre.GetCodec() == "H264")){
				params.MimeType.MimeType = rtp.MIME_TYPE_VIDEO
				if (codecInfo.GetCodec() == "red"){
					params.MimeType.SubType = rtp.SUB_TYPE_RED
				}else if (codecInfo.GetCodec() == "ulpfec"){
					params.MimeType.SubType = rtp.SUB_TYPE_ULPFEC
				}

			}else if (pre.GetCodec() == "opus"){
				params.MimeType.MimeType = rtp.MIME_TYPE_AUDIO
				if (codecInfo.GetCodec() == "red"){
					params.MimeType.SubType = rtp.SUB_TYPE_RED
				}else if (codecInfo.GetCodec() == "ulpfec"){
					params.MimeType.SubType = rtp.SUB_TYPE_ULPFEC
				}
			}

		}

	}
	params.PayloadType = pt
	params.SpatialLayers =1
	params.TemporalLayers =1
	source := this.Info.GetSource(uint(ssrc))
	if (nil != source){
		params.Cname = source.GetCName()
	}


	for _, fb := range fbInfo {
		id := fb.GetID()
		pm := fb.GetParams()
		if (id == "nack") {
			if (len(pm) == 0) {
				params.UseNack = true
			} else {
				if (pm[0] == "pli") {
					params.UsePli = true
				}
			}

		} else if (id == "ccm") {
			if len(pm) == 0 {
				continue
			}

			if (pm[0] == "fir") {
				params.UseFir = true
			}
		}

	}

	v = streamSend.NewStreamSend(streamSend.DeStorageSize, params,this)
	if nil != v{
		//v.MapSsrc = packet.SSRC
		ssrcs,ok := this.mapSSRC[ssrc]
		if ok == true{
			v.MapSsrc = uint32(ssrcs[0])
		}

		this.SendStreamMap[ssrc] = v
		this.rtpStream = v
	}


	return v

}

func (this *SimpleConsumer)OnRtpStreamRetransmitRtpPacket(streamSend *streamSend.StreamSend, packet *rtp.Packet,probation bool){
	this.listener.OnConsumerRetransmitRtpPacket(this, packet, probation)
}


func (this *SimpleConsumer)GetRtcp(packet *compoundrtcp.CompoundRtcp, rtpStream *streamSend.StreamSend,now uint64){
	if (float32(now - this.LastRtcpSentTime) * 1.15 < float32(this.MaxRtcpInterval)){
		return
	}
    if (this.rtpStream != rtpStream){
		mylog.Logger.Errorf("RTP stream does not match\n")
	}
	report := this.rtpStream.GetRtcpSenderReport(now)
	if (nil == report){
		return
	}
	packet.AddSenderReport(report)
	mylog.Logger.Infof("add sr Consumer streamSSRC[%x] string[%s]\n", rtpStream.Params.SSRC, report.String())

	sdesChunk := this.rtpStream.GetRtcpSdesChunk()

	packet.AddSdes(sdesChunk)
	mylog.Logger.Infof("add sdes Consumer streamSSRC[%x] string[%s]\n", rtpStream.Params.SSRC,sdesChunk.String())

	this.LastRtcpSentTime = now

}

func (this *SimpleConsumer)GetTransmissionRate(now uint64) uint32{
	if (nil == this.rtpStream){
		return 0
	}
	return this.rtpStream.GetBitrate(now)
}

func (this *SimpleConsumer)ReceiveNack(nack *rtcp.TransportLayerNack){
	v, ok := this.SendStreamMap[nack.MediaSSRC]
	if ok == true{
		v.ReceiveNack(nack)
	}else{
		mylog.Logger.Error("cant get send stream")
	}
}

func (this *SimpleConsumer)RequestKeyFrame(ssrc uint32){
	if webrtc.RTPCodecTypeVideo != this.Kind{
		return
	}
	this.listener.OnConsumerKeyFrameRequested(this, ssrc)
}

func (this *SimpleConsumer)ReceiveKeyFrameRequestPLI(packet *rtcp.PictureLossIndication){
	streamSend := this.GetStreamSendbySSRC(packet.MediaSSRC)
	if (nil == streamSend) {
		mylog.Logger.Errorf("ReceiveKeyFrameRequestPLI fail  ssrc[%v]", packet.MediaSSRC)
		return
	}
	streamSend.ReceiveKeyFrameRequestPLI()
	this.RequestKeyFrame(streamSend.MapSsrc)
}

func (this *SimpleConsumer)GetStreamSendbySSRC(ssrc uint32)*streamSend.StreamSend{
	v,ok := this.SendStreamMap[ssrc]
	if !ok{
		mylog.Logger.Errorf("Consumer GetStreamSendbySSRC fail  [%v]", ssrc)
		return nil
	}
	return v

}

func (this *SimpleConsumer)ProducerRtpStream(stream *streamRecv.StreamRecv){
	//this.producerRtpStream = stream
}

func (this *SimpleConsumer)GetRtpStreams() map[uint32]*streamSend.StreamSend{
	return this.SendStreamMap
}

func (this *SimpleConsumer)ReceiveRtcpReceiverReport(report rtcp.ReceptionReport){
	if (nil == this.rtpStream){
		return
	}
	this.rtpStream.ReceiveRtcpReceiverReport(report)
}

func (this *SimpleConsumer)ProducerRtcpSenderReport(rtpStream *streamRecv.StreamRecv, first bool){

}

func (this *SimpleConsumer)NeedWorstRemoteFractionLost(mappedSsrc uint32, worstRemoteFractionLost *uint8){

	//if (!IsActive())
	//return;
    if (nil == this.rtpStream){
		return
	}
	 fractionLost := this.rtpStream.FractionLost

	// If our fraction lost is worse than the given one, update it.
	if fractionLost > *worstRemoteFractionLost{
		*worstRemoteFractionLost = fractionLost
	}

}

func (this *SimpleConsumer)GetWriteTrack(pt uint8)(*webrtc.Track){
	v,ok := this.mapWriteRtpTrack[pt]
	if false == ok{
		return nil
	}
	return v
}

func (this *SimpleConsumer)Close(){
	this.Info = nil
	this.Track = nil

	for _,v := range this.SendStreamMap{
		v.Close()
	}
	//close(this.chanRecvRtp)
	//this.chanRecv = nil
	this.SendStreamMap = nil
	this.rtpStream = nil
	this.mapPt = nil
	this.mapWriteRtpTrack = nil
	//this.producerRtpStream = nil

}


func (this *SimpleConsumer)ID() string{
	return this.Id

}

func (this *SimpleConsumer)ProducerClosed(){
	this.listener.OnConsumerProducerClosed(this)
}