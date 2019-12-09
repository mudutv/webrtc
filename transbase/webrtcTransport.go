package transbase

import (
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/utils"
	"github.com/pion/webrtc"
	"github.com/pion/webrtc/compoundrtcp"
	"github.com/pion/webrtc/consumer"
	"github.com/pion/webrtc/deque"
	"github.com/pion/webrtc/mylog"
	"github.com/pion/webrtc/producer"
	"github.com/pion/webrtc/rembServer"
	"github.com/pion/webrtc/uvtime"
	"io"
)

const (
	PUSH_TYPE = iota
	GET_TYPE
)

type WebrtcTransport struct {
	Transport //一定要写在开头
	PeerConnection     *webrtc.PeerConnection
	rembServer         *rembServer.RemoteBitrateEstimatorAbsSendTime
	maxIncomingBitrate uint64
	mixIncomingBitrate uint64
	PCType int

	chanRecvRtcp chan []rtcp.Packet
	chanRecvRtp chan *rtp.Packet
}

func NewWebrtcTransport(id string, listener Listener, pctype int) *WebrtcTransport {
	this := WebrtcTransport{}
	this.NewTransport(id, listener)
	this.listenerChild = &this
	this.PCType = pctype
	this.mixIncomingBitrate = 150000 * 8
	this.chanRecvRtcp = make(chan []rtcp.Packet,200)
	this.chanRecvRtp = make(chan *rtp.Packet,300)

	return &this
}

func (this *WebrtcTransport) Close() {
	this.SetCloseFlag(true)
	mylog.Logger.Infof("RoomTransport streamKey[%s] peerId[%s] close Transport begin",this.listener.OnTransportGetRouterId(), this.Id)
	this.Transport.Close()
	mylog.Logger.Infof("RoomTransport streamKey[%s] peerId[%s] close Transport end",this.listener.OnTransportGetRouterId(),this.Id)
	if (nil != this.PeerConnection) {
		err := this.PeerConnection.Close()
		if nil != err{
			mylog.Logger.Infof("RoomTransport streamKey[%s] peerId[%s] close PeerConnection err[%v]",this.listener.OnTransportGetRouterId(), this.Id,err)
		}else{
			mylog.Logger.Infof("RoomTransport streamKey[%s] peerId[%s] close PeerConnection success",this.listener.OnTransportGetRouterId(), this.Id)
		}
		this.PeerConnection = nil
	}

	if (nil != this.rembServer) {
		this.rembServer.Close()
		this.rembServer = nil
		mylog.Logger.Infof("RoomTransport streamKey[%s] peerId[%s] close rembServer",this.listener.OnTransportGetRouterId(), this.Id)
	}

	close(this.chanRecvRtcp)
	close(this.chanRecvRtp)

}

func (this *WebrtcTransport) IsClose() bool {
	return this.CloseFlag

}


func (this *WebrtcTransport) RtpDataStore(packet *rtp.Packet) bool{
	if (this.IsClose()){
		return false
	}

	if this.chanRecvRtp != nil{
		this.chanRecvRtp <- packet
	}

	return true
}

func (this *WebrtcTransport) OnRtpDataReceived(packet *rtp.Packet) bool{
	if (this.IsClose()){
		return false
	}

	this.DataReceived(len(packet.Raw))
	if (packet.SequenceNumber%500 == 0) {
		mylog.Logger.Infof("streamKey[%s] peerId[%s] add remb rev GetBitrate[%v]\n", this.listener.OnTransportGetRouterId(),this.Id, this.GetRecvBitrate())
	}

	if (this.ICEConnectionState != webrtc.ICEConnectionStateConnected) {
		mylog.Logger.Errorf("ignoring RTP packet while DTLS not connected ssrc[%v] seqnumber[%d] ICEConnectionState[%d]\n", packet.SSRC, packet.SequenceNumber, this.ICEConnectionState)
		return true
	}

	if (nil != this.rembServer) {
		var absSendTime uint32

		//mylog.Logger.Errorf("OnRtpDataReceived abs[%d]\n", this.RtpHeaderExtensionIds.AbsSendTime)
		if (packet.ReadAbsSendTime(this.RtpHeaderExtensionIds.AbsSendTime, &absSendTime)) {
			this.rembServer.IncomingPacket(
				uvtime.GettimeMs(), uint(packet.PayloadLength), packet, absSendTime)
		}
	}

	producer := this.RtpListener.GetProducerbySSRC(packet.SSRC)
	if (nil == producer) {
		mylog.Logger.Errorf("no suitable Producer for received RTP packet ssrc:%v, SequenceNumber:%v PayloadType[%d]]\n", packet.SSRC, packet.SequenceNumber,packet.PayloadType)
		return true
	}

	//mylog.Logger.Errorf("miaobinwei111 kind[%d] ssrc[%v] pt[%d] seq[%d]", producer.Kind, packet.SSRC,packet.PayloadType,packet.SequenceNumber)
	producer.ReceiveRtpPacket(packet)

	return true

}

func (this *WebrtcTransport) OnRtcpDataReceived(packets []rtcp.Packet) bool{
	if (this.IsClose()){
		return false
	}
	//for _, packet := range packets {
	//	if (this.CloseFlag){
	//		return false
	//	}
	//	this.ReceiveRtcpPacket(packet)
	//}
	if (nil != this.chanRecvRtcp ){
		this.chanRecvRtcp <- packets
	}
	return true

}

func (this *WebrtcTransport) SendRtpPacket(packet *rtp.Packet, consumer consumer.InferfaceConsumer, retransmitted bool, probation bool) {
	track := consumer.GetWriteTrack(packet.PayloadType)
	if nil == track{
		mylog.Logger.Errorf("streamKey[%s] peerId[%s] consumer.ID[%s]  ssrc[%v],seq[%v],pt[%d] GetWriteTrack  faile\n", this.listener.OnTransportGetRouterId(), this.Id,consumer.ID(), packet.SSRC, packet.SequenceNumber,packet.PayloadType)
		return
	}
	if err := track.WriteRTP(packet); err != nil && err != io.ErrClosedPipe {
		mylog.Logger.Errorf("streamKey[%s] peerId[%s] consumer.ID[%s]  ssrc[%v],seq[%v] pt[%d] WriteRTP  faile\n", this.listener.OnTransportGetRouterId(), this.Id,consumer.ID(), packet.SSRC, packet.SequenceNumber,packet.PayloadType)
		return
	}
	this.DataSent(packet.RawLen)
	if (packet.SequenceNumber%500 == 0) {
		mylog.Logger.Infof("streamKey[%s] peerId[%s] add remb send GetBitrate[%v]\n", this.listener.OnTransportGetRouterId(),this.Id, this.GetSendBitrate())
	}

}

func (this *WebrtcTransport) SendRtcpPacket(packet []rtcp.Packet) {
	var len int
	len, rtcpSendErr := this.PeerConnection.WriteRTCPLen(packet)
	if rtcpSendErr != nil {
		mylog.Logger.Errorf("producer OnROnRtpStreamSendRtcpPackettpStreamSendRtcpPacket  write rtcp fail [%s]\n", rtcpSendErr.Error())
	}
	this.DataSent(len)
	mylog.Logger.Infof("streamKey[%s] peerId[%s] write rtcp success\n",this.listener.OnTransportGetRouterId(), this.Id)
}

func (this *WebrtcTransport) SendRtcpCompoundPacket(compoundRtcp *compoundrtcp.CompoundRtcp) {
	var len int
	len, rtcpSendErr := this.PeerConnection.WriteRTCPLen(compoundRtcp.Packet)
	if rtcpSendErr != nil {
		mylog.Logger.Errorf("producer OnROnRtpStreamSendRtcpPackettpStreamSendRtcpPacket  write rtcp fail [%s]\n", rtcpSendErr.Error())
	}
	this.DataSent(len)
	mylog.Logger.Infof("streamKey[%s] peerId[%s] write Compoundrtcp success\n",this.listener.OnTransportGetRouterId(), this.Id)
}

func (this *WebrtcTransport) UserOnNewProducer(producer *producer.Producer) {
	rembflag := false

	codecs := producer.GetRtpParameters()
	for _, codec := range codecs {
		for _, fb := range codec.GetRTCPFeedbacks() {
			id := fb.GetID()
			//pm := fb.GetParams()
			if (id == "goog-remb") {
				rembflag = true
				break
			}
		}

		if (true == rembflag) {
			break
		}
	}

	mylog.Logger.Errorf("streamKey[%s] peerId[%s] producerid[%s] remb poin[%v] abssendtime[%d] rembflag[%d]\n", this.listener.OnTransportGetRouterId(), this.Id, producer.Id, this.rembServer, this.RtpHeaderExtensionIds.AbsSendTime, rembflag)

	if (nil == this.rembServer && this.RtpHeaderExtensionIds.AbsSendTime != 0 && true == rembflag) {
		this.rembServer = rembServer.NewRemoteBitrateEstimatorAbsSendTime(this)
	}
}

func (this *WebrtcTransport) OnRembServerAvailableBitrate(remoteBitrateEstimator *rembServer.RemoteBitrateEstimatorAbsSendTime, ssrcs *deque.Deque, availableBitrate uint32) {
	if (this.maxIncomingBitrate != 0) {
		availableBitrate = uint32(utils.MinMy(int64(this.maxIncomingBitrate), int64(availableBitrate)))
	}
	packet := rtcp.ReceiverEstimatedMaximumBitrate{}
	packet.SenderSSRC = 1
	packet.Bitrate = uint64(availableBitrate)
	if (packet.Bitrate < this.mixIncomingBitrate){
		mylog.Logger.Infof("streamKey[%s] peerId[%s] add remb qian[%s] \n", this.listener.OnTransportGetRouterId(), this.Id, packet.String())
		packet.Bitrate = this.mixIncomingBitrate
	}

	packet.SSRCs = make([]uint32, 0, 3)
	num := ssrcs.Len()
	for i := 0; i < num; i++ {
		it := ssrcs.At(i)
		ssrc := it.(uint32)
		packet.SSRCs = append(packet.SSRCs, ssrc)

	}

	//packet.Bitrate = 150000 * 8 * 10
	mylog.Logger.Infof("streamKey[%s] peerId[%s] add remb [%s] \n", this.listener.OnTransportGetRouterId(), this.Id, packet.String())
	this.SendRtcpPacket([]rtcp.Packet{&packet})
}

func (this *WebrtcTransport) ConsumerRecvPacketRun() {
	go func() {
		this.AddWaitGroup(1)
		defer this.DoneWaitGroup()
		for {
			select {
			case rtpPacket := <-this.chanRecvRtp:
				if this.IsClose(){
					mylog.Logger.Infof("RoomTransport streamKey[%s] peerId[%s] ConsumerRecvPacketRun is close\n", this.listener.OnTransportGetRouterId(), this.Id)
					return
				}
				//consumer, ok := this.mapSsrcConsumer[rtpPacket.SSRC]
				consumer:= this.GetConsumerByMediaSsrc(rtpPacket.SSRC)
				if nil != consumer {
					consumer.SendRtpPacketReadyDoing(rtpPacket)
				}else{
					mylog.Logger.Warnf("WebrtcTransport streamKey[%s] peerId[%s] Consumer RecvPacket Run cant get rtp ssrc[%v]\n", this.listener.OnTransportGetRouterId(),this.Id, rtpPacket.SSRC)
				}
			case rtcpPackets := <-this.chanRecvRtcp:
				if this.IsClose(){
					mylog.Logger.Infof("RoomTransport streamKey[%s] peerId[%s] ConsumerRecvPacketRun is close\n", this.listener.OnTransportGetRouterId(), this.Id)
					return
				}
				for _, packet := range rtcpPackets {
					if (this.IsClose()) {
						mylog.Logger.Infof("RoomTransport streamKey[%s] peerId[%s] ConsumerRecvPacketRun is close\n", this.listener.OnTransportGetRouterId(), this.Id)
						return
					}
					this.ReceiveRtcpPacket(packet)
				}
			case <-this.Ctx.Done():
				mylog.Logger.Infof("RoomTransport streamKey[%s] peerId[%s] ConsumerRecvPacketRun ctx done\n", this.listener.OnTransportGetRouterId(), this.Id)
				return
			}

		}
	}()
}

func (this *WebrtcTransport) ProducerRecvPacketRun() {
	go func() {
		this.AddWaitGroup(1)
		defer this.DoneWaitGroup()

		for {
			select {
			case rtpPacket := <-this.chanRecvRtp:
				if this.IsClose(){
					mylog.Logger.Infof("RoomTransport streamKey[%s] peerId[%s] ProducerRecvPacketRun is close\n", this.listener.OnTransportGetRouterId(), this.Id)
					return
				}
				if (false == this.OnRtpDataReceived(rtpPacket)){
					mylog.Logger.Errorf("RoomTransport streamKey[%s] peerId[%s] ProducerRecvPacketRun fail",  this.listener.OnTransportGetRouterId(), this.Id)
					return
				}
			case rtcpPackets := <-this.chanRecvRtcp:
				if this.IsClose(){
					mylog.Logger.Infof("RoomTransport streamKey[%s] peerId[%s] ProducerRecvPacketRun is close\n", this.listener.OnTransportGetRouterId(), this.Id)
					return
				}
				for _, packet := range rtcpPackets {
					if (this.IsClose()) {
						mylog.Logger.Infof("RoomTransport streamKey[%s] peerId[%s] ProducerRecvPacketRun is close\n", this.listener.OnTransportGetRouterId(), this.Id)
						return
					}
					this.ReceiveRtcpPacket(packet)
				}
			case <-this.Ctx.Done():
				mylog.Logger.Infof("RoomTransport streamKey[%s] peerId[%s] ProducerRecvPacketRun ctx done\n", this.listener.OnTransportGetRouterId(), this.Id)
				return
			}

		}

	}()
}