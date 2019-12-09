package serverGrpc

import (
	"errors"
	"fmt"
	"github.com/pion/webrtc"
	"github.com/pion/webrtc/apolloConfig"
	"github.com/pion/webrtc/examples/internal/signal"
	"github.com/pion/webrtc/examples/sfu-my/httpclient"
	pb "github.com/pion/webrtc/examples/sfu-my/proto/rtcserver"
	"github.com/pion/webrtc/examples/sfu-my/room"
	"github.com/pion/webrtc/internal/util"
	"github.com/pion/webrtc/mapsync"
	"github.com/pion/webrtc/mylog"
	"github.com/pion/webrtc/notedit/sdp"
	"github.com/pion/webrtc/router"
	"github.com/pion/webrtc/transbase"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
	"math/rand"
	"net"
	"time"
)

var peerConnectionConfig = []webrtc.Configuration{{ICEServers: []webrtc.ICEServer{
	{
		URLs: []string{"stun:stun.mrtc.myun.tv:3478"},
		//Username: "math",
		//Credential:"ado1x6e65w3l4j3i",
	},
},
},

	{ICEServers: []webrtc.ICEServer{
		{
			URLs: []string{"stun:stun.mrtc.myun.tv:3478"},
			//Username: "math",
			//Credential:"ado1x6e65w3l4j3i",
		},
	},

		SDPSemantics: webrtc.SDPSemanticsPlanB,},
	//ICEServers: []webrtc.ICEServer{
	//	{
	//		URLs: []string{"stun:stun.mrtc.myun.tv:3478","turn:stun.mrtc.myun.tv:3478"},
	//		Username: "math",
	//		Credential:"ado1x6e65w3l4j3i",
	//	},
	//},
}

type consumerNode struct {
	tp       *transbase.WebrtcTransport
	vp8Rtcp  *webrtc.RTPSender
	h264Rtcp *webrtc.RTPSender
	opusRtcp *webrtc.RTPSender

	liveCount int
}

var consumerConnectList *mapsync.MapSync

//var gRPCPort = utils.GetEnvWithDefault("GRPC_RTC_SERVER_PORT", "60000")
//var gRPCPort string = "60000"

type RtcServerImp struct {
	conn pb.RtcInterfaceServer
}

func NewReplyInfo(Result bool, Errmsg string) *pb.ReplyInfo {
	replyInfo := &pb.ReplyInfo{Result: true}
	replyInfo.Result = Result
	replyInfo.Errmsg = Errmsg
	return replyInfo
}

func (*RtcServerImp) SetConfig(ctx context.Context, req *pb.ConfigInfo) (*pb.ReplyInfo, error) {
	return NewReplyInfo(true, ""), nil
}

func CreateOfferDoing(ctx context.Context, req *pb.SdpData) (*pb.ReplyInfo, error) {
	peerId := req.PeerId
	streamKey := req.GetStreamKey()
	mylog.Logger.Infof("CreateOffer streamKey[%s] peerId[%s] begin", streamKey, peerId)

	roomNode := room.RoomGroup.Load(streamKey)
	if nil != roomNode {
		mylog.Logger.Errorf("CreateOffer streamKey[%s] peerId[%s] streamKey exit", streamKey, peerId)
		return NewReplyInfo(false, "streamKey exit"), errors.New("streamKey exit")
	}

	mylog.Logger.Infof("CreateOffer offer get sdp[%s]", req.GetSdp())
	offer := webrtc.SessionDescription{}
	signal.DecodeNoBase64(req.GetSdp(), &offer)
	v, err := sdp.Parse(offer.SDP)
	if err != nil {
		mylog.Logger.Errorf("CreateOffer streamKey[%s] peerId[%s] Parse sdp  fail sdp[%s]", streamKey, peerId, offer.SDP)
		return NewReplyInfo(false, "Parse sdp fail"), errors.New("Parse sdp fail")
	}

	var configuration webrtc.Configuration
	IsB := webrtc.DescriptionIsPlanB(&offer)
	if IsB == true {
		configuration = peerConnectionConfig[1]
	} else {
		configuration = peerConnectionConfig[0]
	}
	mylog.Logger.Infof("CreateOffer streamKey[%s] peerId[%s] configIs[%s] IsB[%v]", streamKey, peerId, configuration.SDPSemantics.String(), IsB)
	var absSendTime int = -1
	mediaInfoVideo := v.GetMedia("video")
	if (nil == mediaInfoVideo) {

	} else {
		for k, v := range mediaInfoVideo.GetExtensions() {
			if (v == "http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time") {
				absSendTime = k
				break
			}
		}

		codecs := make(map[int]*sdp.CodecInfo)
		code := mediaInfoVideo.GetCodec("VP8")
		if (nil != code) {
			code.SetType(webrtc.DefaultPayloadTypeVP8)
			codecs[webrtc.DefaultPayloadTypeVP8] = code
		}

		code = mediaInfoVideo.GetCodec("H264")
		if (nil != code) {
			code.SetType(webrtc.DefaultPayloadTypeH264)
			codecs[webrtc.DefaultPayloadTypeH264] = code
		}

		if 0 == len(codecs) {
			mylog.Logger.Errorf("CreateOffer streamKey[%s] peerId[%s] no H264 vp8", streamKey, peerId)
			return NewReplyInfo(false, " no H264 vp8"), errors.New(" no H264 vp8")
		}

		mediaInfoVideo.SetCodecs(codecs)
	}

	mediaInfoAudio := v.GetMedia("audio")
	if (nil == mediaInfoAudio) {

	} else {
		codecs := make(map[int]*sdp.CodecInfo)
		code := mediaInfoAudio.GetCodec("OPUS")
		if (nil != code) {
			code.SetType(webrtc.DefaultPayloadTypeOpus)
			codecs[webrtc.DefaultPayloadTypeOpus] = code
		}

		if 0 == len(codecs) {
			mylog.Logger.Errorf("CreateOffer streamKey[%s] peerId[%s] no opus", streamKey, peerId)
			return NewReplyInfo(false, "no opus"), errors.New("no opus")
		}

		mediaInfoAudio.SetCodecs(codecs)
	}

	roomNode = router.NewRouter(streamKey)
	var successFlag bool = false
	defer func() {
		if false == successFlag {
			mylog.Logger.Errorf("CreateOffer streamKey[%s] peerId[%s] defer close", streamKey, peerId)
			roomNode.Close()
		}
	}()

	tp := roomNode.GetWebrtcTransportById(peerId)
	if (nil != tp) {
		mylog.Logger.Errorf("CreateOffer streamKey[%s] peerId[%s] is exits", streamKey, peerId)
		return NewReplyInfo(false, "peerId is exits"), errors.New("Parse sdp fail")
	}

	webrtcTp := roomNode.CreateWebrtcTransport(peerId, transbase.PUSH_TYPE)
	if (nil == webrtcTp) {
		mylog.Logger.Errorf("CreateOffer streamKey[%s] peerId[%s] CreateWebrtcTransport fail ", streamKey, peerId)
		return NewReplyInfo(false, "create peer fail"), errors.New("create peer fail")
	}

	// Create a new RTCPeerConnection
	peerConnection, err := room.RoomGroup.Api.NewPeerConnection(configuration)
	if err != nil {
		mylog.Logger.Errorf("CreateOffer streamKey[%s] peerId[%s] create peer fail ", streamKey, peerId)
		return NewReplyInfo(false, "create peer fail"), errors.New("create peer fail")
	}

	webrtcTp.PeerConnection = peerConnection
	webrtcTp.TransportProducer(v)

	ssrc := rand.Uint32()
	id := util.RandSeq(16)
	label := util.RandSeq(16)
	localTrackH264, newTrackErr := peerConnection.NewTrack(webrtc.DefaultPayloadTypeH264, ssrc, id, label)
	////localTrack, newTrackErr := peerConnection.NewTrack(webrtc.DefaultPayloadTypeVP8, rand.Uint32(), util.RandSeq(16), util.RandSeq(16))
	if newTrackErr != nil {
		mylog.Logger.Errorf("CreateOffer streamKey[%s] peerId[%s] new track video H264 fail", streamKey, peerId)
		return NewReplyInfo(false, "new track video fail"), errors.New("new track video fail")
	}
	mylog.Logger.Infof("CreateOffer streamKey[%s] peerId[%s] new track video pt[%v] ssrc[%v] track label[%v] id[%v] Codec[%v]", streamKey, peerId, localTrackH264.PayloadType(), localTrackH264.SSRC(),
		localTrackH264.Label(), localTrackH264.ID(), localTrackH264.Codec())

	peerConnection.AddTrack(localTrackH264)

	//localTrack, newTrackErr := peerConnection.NewTrack(webrtc.DefaultPayloadTypeH264, rand.Uint32(), util.RandSeq(16), util.RandSeq(16))
	localTrack, newTrackErr := peerConnection.NewTrack(webrtc.DefaultPayloadTypeVP8, ssrc, id, label)
	if newTrackErr != nil {
		mylog.Logger.Errorf("CreateOffer streamKey[%s] peerId[%s] new track video vp8 fail", streamKey, peerId)
		return NewReplyInfo(false, "new track video fail"), errors.New("new track video fail")
	}
	mylog.Logger.Infof("CreateOffer streamKey[%s] peerId[%s] new track video pt[%v] ssrc[%v] track label[%v] id[%v] Codec[%v]", streamKey, peerId, localTrack.PayloadType(), localTrack.SSRC(),
		localTrack.Label(), localTrack.ID(), localTrack.Codec())

	peerConnection.AddTrack(localTrack)

	if _, err = peerConnection.AddTransceiver(webrtc.RTPCodecTypeAudio, webrtc.RtpTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly}); err != nil {
		mylog.Logger.Errorf("CreateOffer streamKey[%s] peerId[%s] new track RTPCodecTypeAudio fail", streamKey, peerId)
		return NewReplyInfo(false, "new track audio fail"), errors.New("new track audio fail")
	}

	mylog.Logger.Infof("CreateOffer streamKey[%s] peerId[%s] new track audio success", streamKey, peerId)

	videotr := peerConnection.GetTransceiverByKind(webrtc.RTPCodecTypeVideo)
	if (nil != videotr) {
		videotr.Direction = webrtc.RTPTransceiverDirectionRecvonly
		if (-1 != absSendTime) {
			videotr.ExtMap = append(videotr.ExtMap, fmt.Sprintf("%d http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time", absSendTime))
		}
	} else {
		mylog.Logger.Infof("CreateOffer streamKey[%s] peerId[%s] get video Transceiver faile", streamKey, peerId)
	}

	peerConnection.OnTrack(func(remoteTrack *webrtc.Track, receiver *webrtc.RTPReceiver) {
		webrtcTp.AddWaitGroup(1)
		defer webrtcTp.DoneWaitGroup()
		kindstring := remoteTrack.Kind().String()
		go func() {
			webrtcTp.AddWaitGroup(1)
			defer webrtcTp.DoneWaitGroup()
			for {
				if true == webrtcTp.IsRouterClose() {
					mylog.Logger.Errorf("RoomTransport streamKey[%s] peerId[%s] prodecer read rtp Router is close ", streamKey, peerId, kindstring)
					return
				}
				rtcp, readErr := receiver.ReadRTCPContext(webrtcTp.Ctx)
				if readErr != nil {
					mylog.Logger.Errorf("RoomTransport streamKey[%s] peerId[%s] prodecer read %s rtcp fail err[%v] ", streamKey, peerId, kindstring, readErr)
					return
				}

				if (nil == rtcp) {
					mylog.Logger.Errorf("RoomTransport streamKey[%s] peerId[%s] prodecer read %s rtcp nil ", streamKey, peerId, kindstring)
					return
				}

				if (false == webrtcTp.OnRtcpDataReceived(rtcp)) {
					mylog.Logger.Errorf("RoomTransport streamKey[%s] peerId[%s] prodecer %s OnRtcpDataReceived fail", streamKey, peerId, kindstring)
					return
				}
			}

		}()

		for {
			if true == webrtcTp.IsRouterClose() {
				mylog.Logger.Errorf("RoomTransport streamKey[%s] peerId[%s] prodecer read rtp Router is close ", streamKey, peerId, kindstring)
				return
			}
			r, readErr := remoteTrack.ReadRTPContext(webrtcTp.Ctx)
			if readErr != nil {
				mylog.Logger.Errorf("RoomTransport streamKey[%s] peerId[%s] prodecer read %s rtp fail err[%v]", streamKey, peerId, kindstring, readErr)
				return
			}

			if (nil == r) {
				mylog.Logger.Errorf("RoomTransport streamKey[%s] peerId[%s] prodecer Read %s RTP nil", streamKey, peerId, kindstring)
				return
			}

			if (false == webrtcTp.RtpDataStore(r)) {
				mylog.Logger.Errorf("RoomTransport streamKey[%s] peerId[%s] prodecer %s RtpDataStroeProducer fail", streamKey, peerId, kindstring)
				return
			}

		}
	})

	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		if (webrtc.ICEConnectionStateConnected == connectionState) {
			webrtcTp.Connected()
		}

		mylog.Logger.Infof("Connection producer State has changed [%s]  streamKey[%s] peerId[%s]\n", connectionState.String(), streamKey, peerId)
		if (webrtc.ICEConnectionStateClosed == connectionState || webrtc.ICEConnectionStateFailed == connectionState) {
			ClosePeerDoing(streamKey, peerId)
		}
	})

	err = peerConnection.SetRemoteDescription(offer)
	if err != nil {
		mylog.Logger.Errorf("CreateOffer streamKey[%s] peerId[%s] set offer fail[%v]", streamKey, peerId, err)
		return NewReplyInfo(false, "set offer fail"), errors.New("set offer fail")
	}

	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		mylog.Logger.Errorf("CreateOffer streamKey[%s] peerId[%s] CreateAnswer fail[%s]", streamKey, peerId, err)
		return NewReplyInfo(false, "CreateAnswer fail"), errors.New("CreateAnswer fail")
	}

	mylog.Logger.Infof("CreateOffer streamKey[%s] peerId[%s] answer sdp [%v]", streamKey, peerId, answer)

	err = peerConnection.SetLocalDescription(answer)
	if err != nil {
		mylog.Logger.Errorf("CreateOffer streamKey[%s] peerId[%s] set answer fail", streamKey, peerId)
		return NewReplyInfo(false, "set answer fail"), errors.New("set answer fail")
	}

	httpclient.OnAnswer(apolloConfig.G_Config.BaseUrl, peerId, answer.SDP)
	webrtcTp.ProducerRecvPacketRun()

	room.RoomGroup.Store(streamKey, roomNode)
	successFlag = true
	return NewReplyInfo(true, ""), nil
}
func (*RtcServerImp) CreateOffer(ctx context.Context, req *pb.SdpData) (*pb.ReplyInfo, error) {

	return CreateOfferDoing(ctx, req)
}
func (*RtcServerImp) SetCandidate(ctx context.Context, req *pb.CandidateData) (*pb.ReplyInfo, error) {
	return NewReplyInfo(true, ""), nil
}
func (*RtcServerImp) StartPlay(ctx context.Context, req *pb.PeerInfo) (*pb.ReplyInfo, error) {
	peerId := req.PeerId
	streamKey := req.GetStreamKey()
	mylog.Logger.Infof("StartPlay streamKey[%s] peerId[%s] begin", streamKey, peerId)
	roomNode := room.RoomGroup.Load(streamKey)
	if nil == roomNode {
		mylog.Logger.Errorf("StartPlay streamKey[%s] peerId[%s] get router fail", streamKey, peerId)
		return NewReplyInfo(false, "get  streamKey room  fail"), status.Errorf(codes.Unimplemented, "get  streamKey room  fail")
	}

	if true == roomNode.OnTransportIsRouterClosed() {
		mylog.Logger.Errorf("StartPlay streamKey[%s] peerId[%s]  router is closeing", streamKey, peerId)
		return NewReplyInfo(false, "get  streamKey room  closing"), status.Errorf(codes.Unimplemented, "get  streamKey room  fail")
	}

	tp := roomNode.GetWebrtcTransportById(peerId)
	if (nil != tp) {
		mylog.Logger.Errorf("StartPlay streamKey[%s] peerId[%s] peerId is exits", streamKey, peerId)
		return NewReplyInfo(false, "StartPlay peerId is exits "), status.Errorf(codes.Unimplemented, "get  streamKey room  fail")
	}

	_, ok := consumerConnectList.Load(peerId)
	if true == ok {
		mylog.Logger.Errorf("StartPlay streamKey[%s] peerId[%s] peerId is startplaying", streamKey, peerId)
		return NewReplyInfo(false, "StartPlay peerId is is startplaying "), status.Errorf(codes.Unimplemented, "get  streamKey room  fail")
	}

	webrtcTp1 := roomNode.CreateWebrtcTransportNoStore(peerId, transbase.GET_TYPE)
	if nil == webrtcTp1 {
		mylog.Logger.Errorf("StartPlay streamKey[%s] peerId[%s] CreateWebrtcTransport fail", streamKey, peerId)
		return NewReplyInfo(false, "StartPlay CreateWebrtcTransport fail"), status.Errorf(codes.Unimplemented, "get CreateWebrtcTransport fail")
	}
	consumerConnectNode := &consumerNode{tp: webrtcTp1, liveCount: 0}
	consumerConnectList.Store(peerId, consumerConnectNode)

	var successflag bool = false
	defer func() {
		if false == successflag {
			mylog.Logger.Errorf("StartPlay streamKey[%s] peerId[%s] defer close", streamKey, peerId)
			//roomNode.TransportClose(webrtcTp1)
			consumerConnectList.Delete(peerId)
			tp.SetCloseFlag(true) //因为下面可能造成堵塞，线程抢占，所以直接先置关闭标志位
			tp.Close()
		}
	}()

	peerConnection, err := room.RoomGroup.Api.NewPeerConnection(peerConnectionConfig[1])
	if err != nil {
		mylog.Logger.Errorf("StartPlay streamKey[%s] peerId[%s] create peer fail", streamKey, peerId)
		return NewReplyInfo(false, "create peer fail"), status.Errorf(codes.Unimplemented, "create peer fail")
	}
	webrtcTp1.PeerConnection = peerConnection

	audioFlag := false
	videoFlag := false
	videoFlag, audioFlag = roomNode.IsProducerSupportVideoAudio()
	mylog.Logger.Infof("StartPlay streamKey[%s] peerId[%s] audioflag[%v] videoflag[%v]", streamKey, peerId, audioFlag, videoFlag)

	var audioTrack *webrtc.Track = nil
	if (true == req.Audio && true == audioFlag) {
		var newTrackErr error
		audioTrack, newTrackErr = peerConnection.NewTrack(webrtc.DefaultPayloadTypeOpus, rand.Uint32(), "audio", "pion")
		if newTrackErr != nil {
			mylog.Logger.Errorf("StartPlay streamKey[%s] peerId[%s] new track audio fail", streamKey, peerId)
			return NewReplyInfo(false, "StartPlay new track audio fail"), errors.New("new track audio fail")
		}
		mylog.Logger.Infof("StartPlay streamKey[%s] peerId[%s] new track audio success pt[%v] ssrc[%v] track label[%v] id[%v] Codec[%v]", streamKey, peerId, audioTrack.PayloadType(), audioTrack.SSRC(),
			audioTrack.Label(), audioTrack.ID(), audioTrack.Codec())

		Sender, err := peerConnection.AddTrack(audioTrack)
		if (nil != err) {
			mylog.Logger.Errorf("StartPlay streamKey[%s] peerId[%s] AddTrack audio fail", streamKey, peerId)
			return NewReplyInfo(false, "StartPlay AddTrack audio fail"), errors.New("AddTrack audio fail")
		}
		consumerConnectNode.opusRtcp = Sender
	}

	var videoTrackVP8 *webrtc.Track = nil
	var videoTrackH264 *webrtc.Track = nil
	if (true == req.Video && true == videoFlag) {
		var newTrackErr error

		ssrc := rand.Uint32()

		videoTrackH264, newTrackErr = peerConnection.NewTrack(webrtc.DefaultPayloadTypeH264, ssrc, "video", "pion")
		if newTrackErr != nil {
			mylog.Logger.Errorf("StartPlay streamKey[%s] peerId[%s] new track video fail", streamKey, peerId)
			return NewReplyInfo(false, "StartPlay new track video fail"), errors.New("new track video fail")
		}
		mylog.Logger.Infof("StartPlay streamKey[%s] peerId[%s] new track Video success pt[%v] ssrc[%v] track label[%v] id[%v] Codec[%v]", streamKey, peerId, videoTrackH264.PayloadType(), videoTrackH264.SSRC(),
			videoTrackH264.Label(), videoTrackH264.ID(), videoTrackH264.Codec())

		SenderH264, err := peerConnection.AddTrack(videoTrackH264)
		if (nil != err) {
			mylog.Logger.Errorf("StartPlay streamKey[%s] peerId[%s] AddTrack video fail", streamKey, peerId)
			return NewReplyInfo(false, "StartPlay AddTrack video fail"), errors.New("AddTrack video fail")
		}

		videoTrackVP8, newTrackErr = peerConnection.NewTrack(webrtc.DefaultPayloadTypeVP8, ssrc, "video", "pion")
		//videoTrack, newTrackErr = peerConnection.NewTrack(webrtc.DefaultPayloadTypeH264, rand.Uint32(),"video", "pion")
		if newTrackErr != nil {
			mylog.Logger.Errorf("StartPlay streamKey[%s] peerId[%s] new track video fail", streamKey, peerId)
			return NewReplyInfo(false, "StartPlay new track video fail"), errors.New("new track video fail")
		}
		mylog.Logger.Infof("StartPlay streamKey[%s] peerId[%s] new track Video success pt[%v] ssrc[%v] track label[%v] id[%v] Codec[%v]", streamKey, peerId, videoTrackVP8.PayloadType(), videoTrackVP8.SSRC(),
			videoTrackVP8.Label(), videoTrackVP8.ID(), videoTrackVP8.Codec())

		SenderVP8, err := peerConnection.AddTrack(videoTrackVP8)
		if (nil != err) {
			mylog.Logger.Errorf("StartPlay streamKey[%s] peerId[%s] AddTrack video fail", streamKey, peerId)
			return NewReplyInfo(false, "StartPlay AddTrack video fail"), errors.New("AddTrack video fail")
		}

		var absSendTime uint8
		absSendTime = roomNode.GetProducerVideoAbsTime()

		videotr := peerConnection.GetTransceiverByKind(webrtc.RTPCodecTypeVideo)
		if (nil != videotr) {
			videotr.ExtMap = append(videotr.ExtMap, fmt.Sprintf("%d http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time", absSendTime))
		} else {
			mylog.Logger.Errorf("StartPlay streamKey[%s] peerId[%s] get video Transceiver faile", streamKey, peerId)
		}

		consumerConnectNode.h264Rtcp = SenderH264
		consumerConnectNode.vp8Rtcp = SenderVP8
	}

	offer, err := peerConnection.CreateOffer(nil)
	if (nil != err) {
		mylog.Logger.Errorf("StartPlay streamKey[%s] peerId[%s] create offer fail", streamKey, peerId)
		return NewReplyInfo(false, "create peer fail"), status.Errorf(codes.Unimplemented, "create peer fail")
	}
	mylog.Logger.Infof("StartPlay offer sdp   sdp[%v]", offer)
	err = peerConnection.SetLocalDescription(offer)
	if err != nil {
		mylog.Logger.Errorf("StartPlay streamKey[%s] peerId[%s] SetLocalDescription fail", streamKey, peerId)
		return NewReplyInfo(false, "SetLocalDescription fail"), status.Errorf(codes.Unimplemented, "SetLocalDescription fail")
	}

	offerSdp, err := sdp.Parse(offer.SDP)
	if err != nil {
		mylog.Logger.Errorf("StartPlay streamKey[%s] peerId[%s] Parse offer sdp fail", streamKey, peerId)
		return NewReplyInfo(false, "StartPlay Parse offer sdp fail"), status.Errorf(codes.Unimplemented, "StartPlay Parse offer sdp fail")
	}

	if (nil != videoTrackH264) {
		webrtcTp1.TransportConsumerbyTrack(offerSdp, roomNode.MapProducers, videoTrackH264)
	}

	if (nil != videoTrackVP8) {
		webrtcTp1.TransportConsumerbyTrack(offerSdp, roomNode.MapProducers, videoTrackVP8)
	}

	if (nil != audioTrack) {
		webrtcTp1.TransportConsumerbyTrack(offerSdp, roomNode.MapProducers, audioTrack)
	}

	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		if (webrtc.ICEConnectionStateConnected == connectionState) {
			webrtcTp1.Connected()
		}
		mylog.Logger.Infof("Connection consumer State has changed [%s]  streamKey[%s] peerId[%s]\n", connectionState.String(), streamKey, peerId)
		if (webrtc.ICEConnectionStateClosed == connectionState || webrtc.ICEConnectionStateFailed == connectionState) {
			ClosePeerDoing(streamKey, peerId)
		}
	})

	httpclient.OnOffer(apolloConfig.G_Config.BaseUrl, peerId, offer.SDP)
	successflag = true

	return NewReplyInfo(true, ""), nil
}
func (*RtcServerImp) CreateAnswer(ctx context.Context, req *pb.SdpData) (*pb.ReplyInfo, error) {
	peerId := req.PeerId
	streamKey := req.GetStreamKey()
	mylog.Logger.Errorf("CreateAnswer begin  streamKey[%s] peerId[%s]", streamKey, peerId)
	roomNode := room.RoomGroup.Load(streamKey)
	if nil == roomNode {
		mylog.Logger.Errorf("CreateAnswer get router fail streamKey[%s] peerId[%s]", streamKey, peerId)
		return NewReplyInfo(false, "get  streamKey room  fail"), status.Errorf(codes.Unimplemented, "get  streamKey room  fail")
	}

	if true == roomNode.OnTransportIsRouterClosed() {
		mylog.Logger.Errorf("CreateAnswer streamKey[%s] peerId[%s]  router is closeing", streamKey, peerId)
		return NewReplyInfo(false, "get  streamKey room  closing"), status.Errorf(codes.Unimplemented, "get  streamKey room  fail")
	}

	tp := roomNode.GetWebrtcTransportById(peerId)
	if (nil != tp) {
		mylog.Logger.Errorf("CreateAnswer peerId is exits streamKey[%s] peerId[%s]", streamKey, peerId)
		return NewReplyInfo(false, "CreateAnswer peerId is exits "), status.Errorf(codes.Unimplemented, "CreateAnswer peerId is exits")
	}

	v, ok := consumerConnectList.Load(peerId)
	if (ok != true) {
		mylog.Logger.Errorf("CreateAnswer peerId is no startpalying streamKey[%s] peerId[%s]", streamKey, peerId)
		return NewReplyInfo(false, "CreateAnswer peerId is no startpalying "), status.Errorf(codes.Unimplemented, "CreateAnswer peerId is not no startpalying")
	}

	var successflag bool = false
	defer func() {
		consumerConnectList.Delete(peerId)
		if false == successflag {
			if nil != tp {
				mylog.Logger.Errorf("CreateAnswer streamKey[%s] peerId[%s] defer close", streamKey, peerId)
				tp.SetCloseFlag(true) //因为下面可能造成堵塞，线程抢占，所以直接先置关闭标志位
				tp.Close()
			}
		}
	}()

	conNode := v.(*consumerNode)
	if conNode.tp == nil {
		mylog.Logger.Errorf("CreateAnswer connect Node tp is nil streamKey[%s] peerId[%s]", streamKey, peerId)
		return NewReplyInfo(false, "CreateAnswer connect Node tp is nil "), status.Errorf(codes.Unimplemented, "CreateAnswer connect Node tp is nil")
	}
	tp = conNode.tp

	answer := webrtc.SessionDescription{}
	signal.DecodeNoBase64(req.GetSdp(), &answer)
	mylog.Logger.Infof("CreateAnswer answer sdp   sdp[%v]", answer)

	err := tp.PeerConnection.SetRemoteDescription(answer)
	if err != nil {
		mylog.Logger.Errorf("CreateAnswer SetRemoteDescription fail streamKey[%s] peerId[%s]", streamKey, peerId)
		//roomNode.TransportClose(tp)
		return NewReplyInfo(false, "SetRemoteDescription fail"), status.Errorf(codes.Unimplemented, "SetRemoteDescription fail")
	}

	if (nil != conNode.h264Rtcp) {
		mylog.Logger.Infof("CreateAnswer streamKey[%s] peerId[%s] read Consumer Video H264 rtcp begin \n", streamKey, peerId)
		go func() {
			tp.AddWaitGroup(1)
			defer tp.DoneWaitGroup()
			for {
				if (true == tp.IsRouterClose()) {
					mylog.Logger.Errorf("RoomTransport streamKey[%s] peerId[%s] read Consumer Video H264 rtcp router is close\n", streamKey, peerId)
					return
				}

				v, err := conNode.h264Rtcp.ReadRTCPContext(tp.Ctx)
				if nil != err {
					mylog.Logger.Errorf("RoomTransport streamKey[%s] peerId[%s] read Consumer Video H264 rtcp fail ==>%v \n", streamKey, peerId, err)
					return
				}

				if (nil == v) {
					mylog.Logger.Errorf("RoomTransport streamKey[%s] peerId[%s] read Consumer Video H264 rtcp nil  \n", streamKey, peerId)
					return
				}

				if (false == tp.OnRtcpDataReceived(v)) {
					mylog.Logger.Errorf("RoomTransport streamKey[%s] peerId[%s] read Consumer Video H264 OnRtcpDataReceived false\n", streamKey, peerId)
					return
				}
			}

		}()
	}

	if (nil != conNode.vp8Rtcp) {
		mylog.Logger.Infof("CreateAnswer streamKey[%s] peerId[%s] read Consumer Video vp8 rtcp begin \n", streamKey, peerId)
		go func() {
			tp.AddWaitGroup(1)
			defer tp.DoneWaitGroup()
			for {
				if (true == tp.IsRouterClose()) {
					mylog.Logger.Errorf("RoomTransport streamKey[%s] peerId[%s] read Consumer Video VP8 rtcp router is close\n", streamKey, peerId)
					return
				}

				v, err := conNode.vp8Rtcp.ReadRTCPContext(tp.Ctx)
				if nil != err {
					mylog.Logger.Errorf("RoomTransport streamKey[%s] peerId[%s] read Consumer Video VP8 rtcp fail ==>%v \n", streamKey, peerId, err)
					return
				}

				if (nil == v) {
					mylog.Logger.Errorf("RoomTransport streamKey[%s] peerId[%s] read Consumer Video VP8 rtcp nil  \n", streamKey, peerId)
					return
				}

				if (false == tp.OnRtcpDataReceived(v)) {
					mylog.Logger.Errorf("RoomTransport streamKey[%s] peerId[%s] read Consumer Video VP8 OnRtcpDataReceived false\n", streamKey, peerId)
					return
				}
			}

		}()
	}

	if (nil != conNode.opusRtcp) {
		mylog.Logger.Infof("CreateAnswer streamKey[%s] peerId[%s] read Consumer opus rtcp begin \n", streamKey, peerId)
		go func() {
			tp.AddWaitGroup(1)
			defer tp.DoneWaitGroup()
			for {
				if (true == tp.IsRouterClose()) {
					mylog.Logger.Errorf("RoomTransport streamKey[%s] peerId[%s] read Consumer Audio rtcp router is close\n", streamKey, peerId)
					return
				}

				v, err := conNode.opusRtcp.ReadRTCPContext(tp.Ctx)
				if nil != err {
					mylog.Logger.Errorf("RoomTransport streamKey[%s] peerId[%s] read Consumer Audio rtcp fail ==>%v \n", streamKey, peerId, err)
					return
				}

				if (nil == v) {
					mylog.Logger.Errorf("RoomTransport streamKey[%s] peerId[%s] read Consumer Audio rtcp nil \n", streamKey, peerId)
					return
				}

				if (false == tp.OnRtcpDataReceived(v)) {
					mylog.Logger.Errorf("RoomTransport streamKey[%s] peerId[%s] read Consumer Audio OnRtcpDataReceived false\n", streamKey, peerId)
					return
				}
			}

		}()
	}

	tp.ConsumerRecvPacketRun()

	//毕竟会抢占，插入前再次确认下room是否已经关闭
	roomNode = room.RoomGroup.Load(streamKey)
	if nil == roomNode {
		mylog.Logger.Errorf("CreateAnswer get router fail 2 streamKey[%s] peerId[%s]", streamKey, peerId)
		return NewReplyInfo(false, "get  streamKey room  fail"), status.Errorf(codes.Unimplemented, "get  streamKey room  fail")
	}
	if true == roomNode.OnTransportIsRouterClosed() {
		mylog.Logger.Errorf("CreateAnswer streamKey[%s] peerId[%s]  router is closeing 2", streamKey, peerId)
		return NewReplyInfo(false, "get  streamKey room  closing"), status.Errorf(codes.Unimplemented, "get  streamKey room  fail")
	}
	roomNode.StoreWebrtcTransports(tp)
	successflag = true

	return NewReplyInfo(true, ""), nil
}

func ClosePeerDoing(streamKey string, peerId string) (*pb.ReplyInfo, error) {
	roomNode := room.RoomGroup.Load(streamKey)
	if nil == roomNode {
		mylog.Logger.Errorf("ClosePeer get roomNode fail streamKey[%s] peerId[%s]", streamKey, peerId)
		return NewReplyInfo(false, "no streamKey"), errors.New("no streamKey")
	}

	if true == roomNode.OnTransportIsRouterClosed() {
		mylog.Logger.Errorf("ClosePeer get roomNode streamKey[%s] peerId[%s] room is closeing", streamKey, peerId)
		return NewReplyInfo(false, "room is closeing"), errors.New("room is closeing")
	}

	tp := roomNode.GetWebrtcTransportById(peerId)
	if (nil == tp) {
		mylog.Logger.Errorf("ClosePeer get Transport fail streamKey[%s] peerId[%s]", streamKey, peerId)
		return NewReplyInfo(false, "no peerId"), errors.New("no peerId")
	}

	if transbase.GET_TYPE == tp.PCType {
		//mylog.Logger.Errorf("ClosePeer GET_TYPE streamKey[%s] peerId[%s]", streamKey, peerId)
		//roomNode.TransportClose(tp)
	} else {
		//mylog.Logger.Errorf("ClosePeer push streamKey[%s] peerId[%s]", streamKey, peerId)
		//roomNode.SetCloseFlag(true)
		room.RoomGroup.Delete(streamKey)
		//roomNode.Close()

	}
	roomNode.CloseWaiting(streamKey, peerId)

	return NewReplyInfo(true, ""), nil
}
func (*RtcServerImp) ClosePeer(ctx context.Context, req *pb.PeerInfo) (*pb.ReplyInfo, error) {
	peerId := req.PeerId
	streamKey := req.GetStreamKey()
	mylog.Logger.Errorf("ClosePeer begin  streamKey[%s] peerId[%s]", streamKey, peerId)

	return ClosePeerDoing(streamKey, peerId)
}
func (*RtcServerImp) IsExistStream(ctx context.Context, req *pb.PeerInfo) (*pb.ReplyInfo, error) {
	peerId := req.PeerId
	streamKey := req.GetStreamKey()

	roomNode := room.RoomGroup.Load(streamKey)
	if nil == roomNode {
		mylog.Logger.Errorf("IsExistStream get  roomNode fail streamKey[%s] peerId[%s]", streamKey, peerId)
		return NewReplyInfo(false, "no streamKey"), errors.New("no streamKey")
	}
	return NewReplyInfo(true, ""), nil
}

func RunRtcServer() {
	consumerConnectList = mapsync.NewMapSync()
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		for {
			select {
			case <-ticker.C:
				totalNum := 0
				delmap := make(map[interface{}]int)
				consumerConnectList.Range(func(key, value interface{}) bool {
					node := value.(*consumerNode)
					node.liveCount++
					if (node.liveCount > 2) {
						delmap[key] = node.liveCount
					}
					totalNum++
					return true
				})

				if totalNum > 0 {
					mylog.Logger.Infof("[RunRtcServer] consumerConnectList totalNum  [%d]", totalNum)
				}

				delnum := len(delmap)
				if delnum > 0 {
					mylog.Logger.Warnf("[RunRtcServer] consumerConnectList timeout num [%d]", delnum)
					for k, _ := range delmap {
						mylog.Logger.Warnf("[RunRtcServer] consumerConnectList timeout delete peerId[%s]", k.(string))
						consumerConnectList.Delete(k)
					}
				}
			}
		}

	}()

	listener, err := net.Listen("tcp", "0.0.0.0:"+apolloConfig.G_Config.GrpcPort)
	if err != nil {
		mylog.Logger.Errorf("[RunRtcServer] gRPC failed to listen ip/port[%d]", apolloConfig.G_Config.GrpcPort)
		panic(" [RunRtcServer]gRPC failed to listen ip/port")
	}

	gRPCServer := grpc.NewServer(grpc.KeepaliveParams(keepalive.ServerParameters{
		Time: 10 * time.Second,
	}))
	pb.RegisterRtcInterfaceServer(gRPCServer, &RtcServerImp{})
	if err := gRPCServer.Serve(listener); err != nil {
		mylog.Logger.Errorf("[RunRtcServer] gRPC 服务运行失败！")
		panic("[RunRtcServer] gRPC 服务运行失败！")
	}
}
