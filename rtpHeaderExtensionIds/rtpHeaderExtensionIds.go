package rtpHeaderExtensionIds

const  MaxAudioIntervalMs = 5000
const MaxVideoIntervalMs  = 1000

type RtpHeaderExtensionIds struct {
	Mid               uint8
	Rid               uint8
	Rrid              uint8
	AbsSendTime       uint8
	TransportWideCC01 uint8
	FrameMarking07    uint8 // NOTE: Remove once RFC.
	FrameMarking      uint8
	SsrcAudioLevel    uint8
	VideoOrientation  uint8
	Toffset           uint8
}

func (this *RtpHeaderExtensionIds)InitRtpHeaderExtensionIds(ext map[int]string){
	for k,v := range ext {
		if (v == "http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time"){
			this.AbsSendTime = uint8(k)
		}else if ("urn:ietf:params:rtp-hdrext:sdes:mid" == v){
			this.Mid = uint8(k)
		}else if ("urn:ietf:params:rtp-hdrext:sdes:rtp-stream-id"== v){
			this.Rid =  uint8(k)
		} else if("urn:ietf:params:rtp-hdrext:sdes:repaired-rtp-stream-id" == v){
			this.Rrid =  uint8(k)
		}else if ("http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01" == v){
			this.TransportWideCC01 =  uint8(k)
		}else if ("http://tools.ietf.org/html/draft-ietf-avtext-framemarking-07" == v){
			this.FrameMarking07 =  uint8(k)
		}else if ("urn:ietf:params:rtp-hdrext:framemarking" == v){
			this.FrameMarking =  uint8(k)
		}else if ("urn:ietf:params:rtp-hdrext:ssrc-audio-level" == v){
			this.SsrcAudioLevel =  uint8(k)
		}else if ("urn:3gpp:video-orientation" == v){
			this.VideoOrientation =  uint8(k)
		}else if ("urn:ietf:params:rtp-hdrext:toffset" == v){
			this.Toffset =  uint8(k)
		}else {
			//mylog.Logger.Infof("codecType[%d] fail ext[%d][%s]", codecType, k,v)
		}
	}
}
