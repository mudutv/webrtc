package producer

import (
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/webrtc"
	"github.com/pion/webrtc/compoundrtcp"
	"github.com/pion/webrtc/keyframerequestmanager"
	"github.com/pion/webrtc/mylog"
	"github.com/pion/webrtc/notedit/sdp"
	"github.com/pion/webrtc/rtpHeaderExtensionIds"
	"github.com/pion/webrtc/rtpstream"
	"github.com/pion/webrtc/streamRecv"
)

type Listener interface {
	OnProducerRtpPacketReceived(producer *Producer, packet *rtp.Packet)
	OnProducerSendRtcpPacket(producer *Producer, packet []rtcp.Packet)
	OnProducerRtcpSenderReport(producer *Producer, rtpStream *streamRecv.StreamRecv, first bool)
	OnProducerNeedWorstRemoteFractionLost(producer *Producer, mappedSsrc uint32, worstRemoteFractionLost *uint8)
}

type Producer struct {
	Id                     string
	Kind                   webrtc.RTPCodecType
	mapRtpStreamMappedSsrc map[uint32]*streamRecv.StreamRecv //[ssrc StreamRecv]
	mapRtxSsrcRtpStream    map[uint32]*streamRecv.StreamRecv //[ssrc StreamRecv]
	KeyFrameRequestManager *keyframerequestmanager.KeyFrameRequestManager

	CurrentRtpPacket *rtp.Packet

	maxRtcpInterval  uint16
	lastRtcpSentTime uint64

	//sdp参数
	Info                  *sdp.SDPInfo
	Track                 *sdp.TrackInfo
	RtpHeaderExtensionIds rtpHeaderExtensionIds.RtpHeaderExtensionIds

	listener Listener
}

func NewProucer(track *sdp.TrackInfo, info *sdp.SDPInfo, id string, listener Listener) *Producer {
	//ssrcs := track.GetSSRCS()
	media := track.GetMedia()

	node := Producer{}
	node.Id = id
	node.Track = track
	node.Info = info
	node.mapRtpStreamMappedSsrc = make(map[uint32]*streamRecv.StreamRecv)

	if ("video" == media) {
		node.Kind = webrtc.RTPCodecTypeVideo
		node.KeyFrameRequestManager = keyframerequestmanager.NewKeyFrameRequestManager(&node)
		node.maxRtcpInterval = rtpHeaderExtensionIds.MaxVideoIntervalMs
	} else if ("audio" == media) {
		node.Kind = webrtc.RTPCodecTypeAudio
		node.maxRtcpInterval = rtpHeaderExtensionIds.MaxAudioIntervalMs
	} else {
		mylog.Logger.Errorf("TransportProducer wrong sdp media[%s]", media)
		return nil
	}

	mediainfo := info.GetMedia(media)
	ext := mediainfo.GetExtensions()
	node.RtpHeaderExtensionIds.InitRtpHeaderExtensionIds(ext)

	node.listener = listener
	return &node
}

func (this *Producer) Close() {
	if (nil != this.KeyFrameRequestManager) {
		this.KeyFrameRequestManager.Release()
	}

	for _, v := range this.mapRtpStreamMappedSsrc {
		v.Close()
	}

	this.mapRtpStreamMappedSsrc = nil

	for _, v := range this.mapRtxSsrcRtpStream {
		v.Close()
	}
	this.mapRtxSsrcRtpStream = nil

	this.KeyFrameRequestManager = nil

	this.CurrentRtpPacket = nil
	this.Info = nil
	this.Track = nil

}
//lister KeyFrameRequestManager
func (this *Producer) OnKeyFrameNeeded(KeyFrameRequestManager *keyframerequestmanager.KeyFrameRequestManager, ssrc uint32) {
	streamRecv, ok := this.mapRtpStreamMappedSsrc[ssrc]
	if (!ok) {
		mylog.Logger.Errorf("producer no StreamRecv ssrc[%v]", ssrc)
		return
	}
	mylog.Logger.Infof("OnKeyFrameNeeded send pli  ssrc[%v]", ssrc)
	streamRecv.RequestKeyFrame()
}

func (this *Producer) GetRtpStreamRecvbyRTXPt(pt uint8) *streamRecv.StreamRecv {
	for _, v :=  range this.mapRtpStreamMappedSsrc{
		if (v.Params.RtxPayloadType == pt){
			return v
		}
	}

	return nil
}

func (this *Producer) GetRtpStreamRecv(packet *rtp.Packet) *streamRecv.StreamRecv {
	ssrc := packet.SSRC

	v, ok := this.mapRtpStreamMappedSsrc[ssrc]
	if ok {
		return v
	}

	v, ok = this.mapRtxSsrcRtpStream[ssrc]
	if ok {
		return v
	}

	mediaInfo := this.Info.GetMedia(this.Track.GetMedia())
	if (nil == mediaInfo) {
		return nil
	}

	codecInfo := mediaInfo.GetCodecForType(int(packet.PayloadType))
	if (nil != codecInfo) {
		fbInfo := codecInfo.GetRTCPFeedbacks()

		//fmt.Println("==========================")
		//for _,v := range codecInfo.GetRTCPFeedbacks(){
		//	fmt.Println(v)
		//}
		//fmt.Println("==========================")

		//params := rtpstream.Params{ssrc, 90000, packet.PayloadType}
		params := rtpstream.Params{}
		params.SSRC = packet.SSRC
		if (codecInfo.GetCodec() == "VP8") {
			params.ClockRate = uint32(codecInfo.GetRate())
			params.MimeType.MimeType = rtp.MIME_TYPE_VIDEO
			params.MimeType.SubType = rtp.SUB_TYPE_VP8
		} else if (codecInfo.GetCodec() == "H264") {
			params.ClockRate = uint32(codecInfo.GetRate())
			params.MimeType.MimeType = rtp.MIME_TYPE_VIDEO
			params.MimeType.SubType = rtp.SUB_TYPE_H264
		} else if (codecInfo.GetCodec() == "opus"){
			params.ClockRate = uint32(codecInfo.GetRate())
			params.MimeType.MimeType = rtp.MIME_TYPE_AUDIO
			params.MimeType.SubType = rtp.SUB_TYPE_OPUS
		}else if(codecInfo.GetCodec() == "red" || (codecInfo.GetCodec() == "ulpfec")){
			var pre *sdp.CodecInfo

			for  _,v := range mediaInfo.GetCodecs(){

				if (int(packet.PayloadType) == v.GetType()){
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
		params.PayloadType = packet.PayloadType
		params.RtxPayloadType = uint8(codecInfo.GetRTX())
		params.SpatialLayers = 1
		params.TemporalLayers = 1

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

		v = streamRecv.NewStreamRecv(params, this)
		if nil != v {
			this.mapRtpStreamMappedSsrc[ssrc] = v
		}
	}else {
		mylog.Logger.Infof("new StreamRecv rtx pt[%d] ssrc[%I64u]",packet.PayloadType,packet.SSRC)
		v = this.GetRtpStreamRecvbyRTXPt(packet.PayloadType)
		if (nil == v){
			mylog.Logger.Errorf("ignoring RTX packet for not yet created RtpStream (ssrc lookup)")
			return nil
		}
		v.Params.RtxSsrc = packet.SSRC
		this.mapRtxSsrcRtpStream[ssrc] = v

	}

	return v

}

func (this *Producer) GetRtpStreamRecvbySSRC(ssrc uint32) *streamRecv.StreamRecv {
	v, ok := this.mapRtpStreamMappedSsrc[ssrc]
	if !ok {
		mylog.Logger.Errorf("Producer GetStreamSendbySSRC fail  [%v]", ssrc)
		return nil
	}

	return v

}

func (this *Producer) ReceiveRtpPacket(packet *rtp.Packet) bool {
	this.CurrentRtpPacket = nil
	numRtpStreamsBefore := len(this.mapRtpStreamMappedSsrc)

	rtpStream := this.GetRtpStreamRecv(packet)
	if (nil == rtpStream) {
		mylog.Logger.Errorf("no stream found for received packet ssrc[%v]\n", packet.SSRC)
		return false
	}

	if (rtpStream.GetSsrc() == packet.SSRC){
		// Media packet.
		if (!rtpStream.ReceivePacket(packet)) {
			mylog.Logger.Errorf("ReceivePacket packet fail ssrc[%v] seq[%v]\n", packet.SSRC, packet.SequenceNumber)
			return false
		}

	}else if(rtpStream.GetRtxSsrc() == packet.SSRC){
		// Process the packet.
		if (!rtpStream.ReceiveRtxPacket(packet)){
			return false
		}
	}else {
		mylog.Logger.Errorf("found stream does not match received packet ssrc[%v]", packet.SSRC)
	}



	if (packet.IsKeyFrame()) {
		mylog.Logger.Infof("key frame received [ssrc:%v, seq:%v", packet.SSRC, packet.SequenceNumber)

		// Tell the keyFrameRequestManager.
		if (nil != this.KeyFrameRequestManager) {
			this.KeyFrameRequestManager.KeyFrameReceived(packet.SSRC)
		}
	}

	if (len(this.mapRtpStreamMappedSsrc) > numRtpStreamsBefore) {
		if ((nil != this.KeyFrameRequestManager) && (false == packet.IsKeyFrame())) {
			this.KeyFrameRequestManager.ForceKeyFrameNeeded(packet.SSRC)
		}

		//// Update current packet.
		//this->currentRtpPacket = packet;
		//
		//NotifyNewRtpStream(rtpStream);
		//
		//// Reset current packet.
		//this->currentRtpPacket = nullptr;
	}

	// Mangle the packet before providing the listener with it.
	//mylog.Logger.Errorf("miaobinwei111 kind[%d] ssrc[%v] pt[%d] seq[%d]", this.Kind, packet.SSRC,packet.PayloadType,packet.SequenceNumber)
	this.listener.OnProducerRtpPacketReceived(this, packet)

	return true

}

//lister StreamRecv
func (this *Producer) OnRtpStreamSendRtcpPacket(streamRecv *streamRecv.StreamRecv, packet []rtcp.Packet) {
	this.listener.OnProducerSendRtcpPacket(this, packet)
}

func (this *Producer) RequestKeyFrame(mappedSsrc uint32) {
	recv := this.GetRtpStreamRecvbySSRC(mappedSsrc)
	if (nil == recv) {
		mylog.Logger.Infof("RequestKeyFrame 1111\n")
		return
	}
	this.KeyFrameRequestManager.KeyFrameNeeded(mappedSsrc)
}

func (this *Producer) GetRtcp(packet *compoundrtcp.CompoundRtcp, now uint64) {
	if (float32(now-this.lastRtcpSentTime)*1.15 < float32(this.maxRtcpInterval)) {
		return
	}

	rrpacket := rtcp.ReceiverReport{}
	for _, StreamRecv := range this.mapRtpStreamMappedSsrc {
		report := StreamRecv.GetRtcpReceiverReport()
		rrpacket.Reports = append(rrpacket.Reports, report)
	}
	mylog.Logger.Infof("add rr Producer streamSSRC[%x] string[%s]\n", rrpacket.SSRC, rrpacket.String())
	packet.AddReceiverReport(&rrpacket)
	//这里要添加rtcp xr里面的rrt包，对应收rtcp xr的dlrr包，通过rrt，dlrr计算rtt
	// rtt（往返时延。 在计算机网络中它是一个重要的性能指标，表示从发送端发送数据开始，到发送端收到来自接收端的确认（接收端收到数据后便立即发送确认），总共经历的时延。）
	//因为目前没用到，所以先不做
	{

	}
	//mylog.Logger.Errorf("add rr  [%v]\n", rrpacket)
	this.lastRtcpSentTime = now

}

func (this *Producer) GetRtpParameters() map[int]*sdp.CodecInfo {
	mediainfo := this.Info.GetMedia(this.Track.GetMedia())
	if (nil != mediainfo) {
		return mediainfo.GetCodecs()
	}

	return nil
}

func (this *Producer) ReceiveRtcpSenderReport(sr *rtcp.SenderReport) {
	rtpStream, ok := this.mapRtpStreamMappedSsrc[sr.SSRC]
	if (false == ok) {
		mylog.Logger.Errorf("ReceiveRtcpSenderReport Id[%s] kind[%s] RtpStream not found  [%v]\n", this.Id,this.Kind.String(),sr.SSRC)
		return
	}

	first := (0 == rtpStream.GetSenderReportNtpMs())
	rtpStream.ReceiveRtcpSenderReport(sr)
	this.listener.OnProducerRtcpSenderReport(this, rtpStream, first)
}

func (this *Producer) GetRtpStreams() map[uint32]*streamRecv.StreamRecv {
	return this.mapRtpStreamMappedSsrc
}

func (this *Producer)OnRtpStreamNeedWorstRemoteFractionLost(rtpStream *streamRecv.StreamRecv, worstRemoteFractionLost *uint8){
	//auto mappedSsrc = this->mapRtpStreamMappedSsrc.at(rtpStream);

	// Notify the listener.
	this.listener.OnProducerNeedWorstRemoteFractionLost(this, rtpStream.Params.SSRC, worstRemoteFractionLost)
}
