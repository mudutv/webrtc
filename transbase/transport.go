package transbase

import (
	"context"
	"github.com/pion/alex023/clock"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/webrtc"
	"github.com/pion/webrtc/compoundrtcp"
	"github.com/pion/webrtc/consumer"
	"github.com/pion/webrtc/jmcvetta/randutil"
	"github.com/pion/webrtc/mapsync"
	"github.com/pion/webrtc/mylog"
	"github.com/pion/webrtc/notedit/sdp"
	"github.com/pion/webrtc/producer"
	"github.com/pion/webrtc/rateCalculator"
	"github.com/pion/webrtc/rtpHeaderExtensionIds"
	"github.com/pion/webrtc/rtpListener"
	"github.com/pion/webrtc/streamRecv"
	"github.com/pion/webrtc/uvtime"
	"sync"
	"time"
)

const MtuSize = 1500

type ListenerChild interface {
	SendRtpPacket(packet *rtp.Packet, consumer consumer.InferfaceConsumer, retransmitted bool, probation bool)
	RtpDataStore(packet *rtp.Packet) bool
	SendRtcpPacket(packet []rtcp.Packet)
	SendRtcpCompoundPacket(compoundRtcp *compoundrtcp.CompoundRtcp)
	UserOnNewProducer(producer *producer.Producer)
}

type Listener interface {
	OnTransportNewProducer(transport *Transport, producer *producer.Producer)
	OnTransportNewConsumer(transport *Transport, consumer consumer.InferfaceConsumer, producerId string)
	OnTransportProducerRtpPacketReceived(transport *Transport, producer *producer.Producer, packet *rtp.Packet)
	OnTransportConsumerKeyFrameRequested(transport *Transport, consumer consumer.InferfaceConsumer, mappedSsrc uint32)
	OnTransportProducerRtcpSenderReport(tansport *Transport, producer *producer.Producer, rtpStream *streamRecv.StreamRecv, first bool)
	OnTransportNeedWorstRemoteFractionLost(tansport *Transport, producer *producer.Producer, mappedSsrc uint32, worstRemoteFractionLost *uint8)
	OnTransportProducerClosed(transport *Transport, producer *producer.Producer)
	OnTransportConsumerProducerClosed(transport *Transport, consumer consumer.InferfaceConsumer)
	OnTransportConsumerClosed(tansport *Transport, consumer consumer.InferfaceConsumer)
	OnTransportIsRouterClosed() bool
	OnTransportGetRouterId() string
}

type Transport struct {
	MapProducers map[string]*producer.Producer
	MapConsumers map[string]consumer.InferfaceConsumer

	RtpListener     *rtpListener.RtpListener
	//mapSsrcConsumer map[uint32]consumer.InferfaceConsumer //ssrc   因为这里涉及到多携程操作，所以搞成线程安全的map
	mapSsrcConsumer *mapsync.MapSync //map[uint32]consumer.InferfaceConsumer
	listener      Listener
	listenerChild ListenerChild
	clock         *clock.Clock
	job           clock.Job
	Id            string

	RtpHeaderExtensionIds rtpHeaderExtensionIds.RtpHeaderExtensionIds

	RecvTransmission rateCalculator.RateCalculator //不知道作用，先不做
	SendTransmission rateCalculator.RateCalculator //不知道作用，先不做

	ICEConnectionState webrtc.ICEConnectionState

	CloseFlag bool

	Ctx context.Context
	Cancel context.CancelFunc
	wg sync.WaitGroup

}

func (this *Transport) NewTransport(id string, listener Listener) {
	this.MapProducers = make(map[string]*producer.Producer)
	this.MapConsumers = make(map[string]consumer.InferfaceConsumer)
	//this.mapSsrcConsumer = make(map[uint32]consumer.InferfaceConsumer)
	this.mapSsrcConsumer = mapsync.NewMapSync()

	this.clock = clock.NewClock()
	this.Id = id
	this.listener = listener
	this.RtpListener = rtpListener.NewRtpListener()
	this.RecvTransmission = rateCalculator.NewRateCalculator(0, 0)
	this.SendTransmission = rateCalculator.NewRateCalculator(0, 0)
	this.CloseFlag = false
	this.ICEConnectionState = webrtc.ICEConnectionStateNew

	this.Ctx, this.Cancel = context.WithCancel(context.Background())
}

func (this *Transport) TransportProducer(info *sdp.SDPInfo) {
	streams := info.GetStreams()
	for _, stream := range streams {
		Tracks := stream.GetTracks()
		for _, track := range Tracks {
			producer := producer.NewProucer(track, info, track.GetID(), this)
			this.RtpListener.AddProducer(producer)

			this.listener.OnTransportNewProducer(this, producer)
			this.MapProducers[track.GetID()] = producer

			producerRtpHeaderExtensionIds := producer.RtpHeaderExtensionIds

			if (producerRtpHeaderExtensionIds.Mid != 0) {
				this.RtpHeaderExtensionIds.Mid = producerRtpHeaderExtensionIds.Mid
			}

			if (producerRtpHeaderExtensionIds.Rid != 0) {
				this.RtpHeaderExtensionIds.Rid = producerRtpHeaderExtensionIds.Rid
			}

			if (producerRtpHeaderExtensionIds.Rrid != 0) {
				this.RtpHeaderExtensionIds.Rrid = producerRtpHeaderExtensionIds.Rrid
			}

			if (producerRtpHeaderExtensionIds.AbsSendTime != 0) {
				this.RtpHeaderExtensionIds.AbsSendTime = producerRtpHeaderExtensionIds.AbsSendTime
			}
			// Tell the subclass.设置remb初始化
			this.listenerChild.UserOnNewProducer(producer)
			//mylog.Logger.Infof("peerId[%s] create Producer[%s] success\n", this.Id,producer.Id)
		}
	}

}


//func (this *Transport) TransportConsumerbyTrack(info *sdp.SDPInfo, MapProducers map[string]*producer.Producer, writeRtpTrack *webrtc.Track) {
//	var producerId string
//	var producerinfo *sdp.SDPInfo
//	var producertrack *sdp.TrackInfo
//	if (webrtc.RTPCodecTypeVideo == writeRtpTrack.Kind()) {
//		//mylog.Logger.Errorf("TransportConsumer video \n")
//		for _, producer := range MapProducers {
//			if webrtc.RTPCodecTypeVideo == producer.Kind {
//				producerId = producer.Id
//				producerinfo = producer.Info
//				producertrack = producer.Track
//				break
//			}
//
//		}
//	} else {
//		for _, producer := range MapProducers {
//			if webrtc.RTPCodecTypeAudio == producer.Kind {
//				producerId = producer.Id
//				producerinfo = producer.Info
//				producertrack = producer.Track
//				break
//			}
//
//		}
//	}
//
//	if 0 == len(producerId) {
//		mylog.Logger.Errorf("can not get producer Media [%s] Codec[%s]\n", writeRtpTrack.Kind().String(),writeRtpTrack.Codec().Name)
//		return
//	}
//
//	var track *sdp.TrackInfo
//	streams := info.GetStreams()
//	for _, stream := range streams {
//		Tracks := stream.GetTracks()
//		for _, track1 := range Tracks {
//			if (writeRtpTrack.Kind().String() == track1.GetMedia()) {
//				track = track1
//				break
//			}
//		}
//
//		if (nil != track) {
//			break
//		}
//	}
//
//	mylog.Logger.Infof("TransportConsumer  producerId[%s] GetID[%s] GetMediaID[%s] GetSSRCS[%v] GetMedia[%s]\n", producerId, track.GetID(), track.GetMediaID(), track.GetSSRCS(), track.GetMedia())
//	//track := info.GetTrackByMediaID(writeRtpTrack.Label())
//	//if (track == nil){
//	//	panic("miaobinwei")
//	//}
//	Simpleconsumer, ok := this.MapConsumers[track.GetID()]
//	if false == ok{
//		Simpleconsumer = consumer.NewSimpleConsumer(track, info, this, track.GetID())
//		this.MapConsumers[track.GetID()] = Simpleconsumer
//		for _, ssrc := range Simpleconsumer.GetMediaSsrcs() {
//			//this.mapSsrcConsumer[ssrc] = Simpleconsumer
//			this.mapSsrcConsumer.Store(ssrc, Simpleconsumer)
//		}
//		this.listener.OnTransportNewConsumer(this, Simpleconsumer, producerId)
//	}
//
//	Simpleconsumer.SetMapSSRC(producertrack, writeRtpTrack.SSRC())
//	Simpleconsumer.SetMapPt(producerinfo, writeRtpTrack.Codec().Name, writeRtpTrack.PayloadType())
//	Simpleconsumer.AddwriteTrack(writeRtpTrack.PayloadType(), writeRtpTrack)
//
//
//	mylog.Logger.Infof("TransportConsumer  MapConsumers len[%d]\n", len(this.MapConsumers))
//
//	// Tell the subclass.设置remb初始化
//	//UserOnNewConsumer(producer);
//
//	//如果transport链接成功，直接请求一个关键帧，这里目前不需要
//	//if (IsConnected())
//	//consumer->TransportConnected();
//}

func (this *Transport) TransportConsumerbyTrack(info *sdp.SDPInfo, MapProducers *mapsync.MapSync, writeRtpTrack *webrtc.Track) {
	var producerId string
	var producerinfo *sdp.SDPInfo
	var producertrack *sdp.TrackInfo

	MapProducers.Range(func(key, value interface{}) bool {
		producer := value.(*producer.Producer)
		if writeRtpTrack.Kind() == producer.Kind {
			producerId = producer.Id
			producerinfo = producer.Info
			producertrack = producer.Track
			return false
		}
		return true
	})
	//if (webrtc.RTPCodecTypeVideo == writeRtpTrack.Kind()) {
	//	//mylog.Logger.Errorf("TransportConsumer video \n")
	//	for _, producer := range MapProducers {
	//		if webrtc.RTPCodecTypeVideo == producer.Kind {
	//			producerId = producer.Id
	//			producerinfo = producer.Info
	//			producertrack = producer.Track
	//			break
	//		}
	//
	//	}
	//} else {
	//	for _, producer := range MapProducers {
	//		if webrtc.RTPCodecTypeAudio == producer.Kind {
	//			producerId = producer.Id
	//			producerinfo = producer.Info
	//			producertrack = producer.Track
	//			break
	//		}
	//
	//	}
	//}

	if 0 == len(producerId) {
		mylog.Logger.Errorf("can not get producer Media [%s] Codec[%s]\n", writeRtpTrack.Kind().String(),writeRtpTrack.Codec().Name)
		return
	}

	var track *sdp.TrackInfo
	streams := info.GetStreams()
	for _, stream := range streams {
		Tracks := stream.GetTracks()
		for _, track1 := range Tracks {
			if (writeRtpTrack.Kind().String() == track1.GetMedia()) {
				track = track1
				break
			}
		}

		if (nil != track) {
			break
		}
	}

	mylog.Logger.Infof("TransportConsumer  producerId[%s] GetID[%s] GetMediaID[%s] GetSSRCS[%v] GetMedia[%s]\n", producerId, track.GetID(), track.GetMediaID(), track.GetSSRCS(), track.GetMedia())
	//track := info.GetTrackByMediaID(writeRtpTrack.Label())
	//if (track == nil){
	//	panic("miaobinwei")
	//}
	Simpleconsumer, ok := this.MapConsumers[track.GetID()]
	if false == ok{
		Simpleconsumer = consumer.NewSimpleConsumer(track, info, this, track.GetID())
		this.MapConsumers[track.GetID()] = Simpleconsumer
		for _, ssrc := range Simpleconsumer.GetMediaSsrcs() {
			//this.mapSsrcConsumer[ssrc] = Simpleconsumer
			this.mapSsrcConsumer.Store(ssrc, Simpleconsumer)
		}
		this.listener.OnTransportNewConsumer(this, Simpleconsumer, producerId)
	}

	Simpleconsumer.SetMapSSRC(producertrack, writeRtpTrack.SSRC())
	Simpleconsumer.SetMapPt(producerinfo, writeRtpTrack.Codec().Name, writeRtpTrack.PayloadType())
	Simpleconsumer.AddwriteTrack(writeRtpTrack.PayloadType(), writeRtpTrack)


	mylog.Logger.Infof("TransportConsumer  MapConsumers len[%d]\n", len(this.MapConsumers))

	// Tell the subclass.设置remb初始化
	//UserOnNewConsumer(producer);

	//如果transport链接成功，直接请求一个关键帧，这里目前不需要
	//if (IsConnected())
	//consumer->TransportConnected();
}

func (this *Transport) Connected() bool{
	if (this.ICEConnectionState == webrtc.ICEConnectionStateConnected){
		return false
	}
	job, ok := this.clock.AddJobWithInterval(rtpHeaderExtensionIds.MaxVideoIntervalMs*time.Millisecond/2, this.ontime)
	if !ok {
		mylog.Logger.Errorf("Connected  AddJobWithInterval fail ssrc \n")
		return false
	}
	this.job = job

	this.ICEConnectionState = webrtc.ICEConnectionStateConnected
	mylog.Logger.Errorf("Connected success \n")

	// Tell all Consumers.

	// Tell all DataConsumers.

	// Tell the SctpAssociation.

	return true

}
func (this *Transport) ontime() {
	this.AddWaitGroup(1)
	defer this.DoneWaitGroup()

	//select {
	//case <-this.Ctx.Done():
	//	mylog.Logger.Errorf("ontime ctx done Id[%s] ", this.Id)
	//	return
	//default:
	//}
	if true == this.CloseFlag{
		mylog.Logger.Errorf("RoomTransport streamKey[%s] peerId[%s]  ontime  CloseFlag return", this.listener.OnTransportGetRouterId(),this.Id)
		return
	}

	var interval uint64 = rtpHeaderExtensionIds.MaxVideoIntervalMs
	now := uint64(uvtime.GettimeMs())
	this.SendRtcp(now)

	if (len(this.MapConsumers) > 0) {
		var rate uint32

		for _, consumer := range this.MapConsumers {
			rate += consumer.GetTransmissionRate(now) / 1000
			//mylog.Logger.Errorf("rate[%v]\n",rate)
		}

		if rate != 0 {
			interval = uint64(360000 / rate)
			//mylog.Logger.Errorf("interval[%v]\n",interval)
		}

		if (interval > rtpHeaderExtensionIds.MaxVideoIntervalMs) {
			interval = rtpHeaderExtensionIds.MaxVideoIntervalMs
		}
	}

	randNum, _ := randutil.IntRange(5, 15)
	interval = uint64(float64(interval) * float64(randNum) / 10.0)

	if (0 == interval) {
		mylog.Logger.Errorf("RoomTransport streamKey[%s] peerId[%s] ontime is 0 \n",this.listener.OnTransportGetRouterId(), this.Id)
		interval = rtpHeaderExtensionIds.MaxVideoIntervalMs
	}

	//this.job.Cancel()
	this.job = nil

	if true == this.CloseFlag{
		mylog.Logger.Errorf("RoomTransport streamKey[%s] peerId[%s]  ontime  CloseFlag 2 return", this.listener.OnTransportGetRouterId(), this.Id)
		return
	}
	job, ok := this.clock.AddJobWithInterval(time.Duration(interval)*time.Millisecond, this.ontime)
	if !ok {
		mylog.Logger.Errorf("Restart  ontime fail time[%v]\n", time.Duration(interval)*time.Millisecond)

	}else{
		this.job = job
	}


}
func (this *Transport) SendRtcp(now uint64) {
	packet := compoundrtcp.NewCompoundRtcp()

	for _, consumer := range this.MapConsumers {
		for _, sendstream := range consumer.GetRtpStreams() {
			consumer.GetRtcp(packet, sendstream, now)
			if (len(packet.Packet) > 0) {
				this.listenerChild.SendRtcpCompoundPacket(packet)
			}
			packet = compoundrtcp.NewCompoundRtcp()
		}
	}

	for _, producer := range this.MapProducers {
		producer.GetRtcp(packet, now)
		if (packet.DateLen > MtuSize) {
			this.listenerChild.SendRtcpCompoundPacket(packet)
			packet = compoundrtcp.NewCompoundRtcp()
		}

	}
	//mylog.Logger.Errorf("44len [%v]\n",len(packet.Packet))
	if (len(packet.Packet) > 0) {
		this.listenerChild.SendRtcpCompoundPacket(packet)
	}
}

func (this *Transport) DataReceived(len int) {
	this.RecvTransmission.Update(uint64(len), uint64(uvtime.GettimeMs()))
}

func (this *Transport) DataSent(len int) {
	this.SendTransmission.Update(uint64(len), uint64(uvtime.GettimeMs()))
}

func (this *Transport) GetReceivedBytes() uint64 {
	return this.RecvTransmission.GetBytes()
}

func (this *Transport) GetSentBytes() uint64 {
	return this.SendTransmission.GetBytes()
}

func (this *Transport) GetRecvBitrate() uint32 {
	return this.RecvTransmission.GetRate(uint64(uvtime.GettimeMs()))
}

func (this *Transport) GetSendBitrate() uint32 {
	return this.SendTransmission.GetRate(uint64(uvtime.GettimeMs()))
}

func (this *Transport) OnProducerRtpPacketReceived(producer *producer.Producer, packet *rtp.Packet) {
	this.listener.OnTransportProducerRtpPacketReceived(this, producer, packet)
}

func (this *Transport) OnConsumerSendRtpPacket(consumer consumer.InferfaceConsumer, packet *rtp.Packet) {
	this.listenerChild.SendRtpPacket(packet, consumer, false, false)
}

func (this *Transport) OnConsumerRecvRtpPacket(packet *rtp.Packet){
	this.listenerChild.RtpDataStore(packet)
}

func (this *Transport) OnConsumerRetransmitRtpPacket(consumer consumer.InferfaceConsumer, packet *rtp.Packet, probation bool) {
	// Update http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time if present.
	//{
	//	uint8_t extenLen;
	//	uint8_t* extenValue = packet->GetExtension(
	//		static_cast<uint8_t>(RTC::RtpHeaderExtensionUri::Type::ABS_SEND_TIME), extenLen);
	//
	//	if (extenValue && extenLen == 3)
	//	{
	//		auto now         = DepLibUV::GetTime();
	//		auto absSendTime = static_cast<uint32_t>(((now << 18) + 500) / 1000) & 0x00FFFFFF;
	//
	//	Utils::Byte::Set3Bytes(extenValue, 0, absSendTime);
	//	}
	//}
	this.listenerChild.SendRtpPacket(packet, consumer, true, probation)

}

func (this *Transport) ReceiveRtcpPacket(packet rtcp.Packet) {
	switch t := packet.(type) {
	case *rtcp.SenderReport:
		//fmt.Printf("get Consumer SenderReport ssrc %lu\n",packet.DestinationSSRC())
		mylog.Logger.Infof("get rtcp SenderReport streamKey[%s] peerId[%s] SSRC[%v] string[%s] \n", this.listener.OnTransportGetRouterId(), this.Id,t.SSRC, t.String())
		producer := this.RtpListener.GetProducerbySSRC(t.SSRC)
		if (nil == producer) {
			mylog.Logger.Errorf("no Producer found for received Sender Report [ssrc:%v]\n", t.SSRC)
			break
		}

		producer.ReceiveRtcpSenderReport(t)

	case *rtcp.ReceiverReport:
		mylog.Logger.Infof("get rtcp ReceiverReport streamKey[%s] peerId[%s] peerId[%s] SSRC[%v] string[%s]\n",  this.listener.OnTransportGetRouterId(),this.Id, t.SSRC, t.String())
		for _, report := range t.Reports {
			consumer := this.GetConsumerByMediaSsrc(report.SSRC)
			if (nil == consumer) {
				mylog.Logger.Errorf("no Consumer found for received Receiver Report [ssrc:%v]\n", report.SSRC)
				continue
			}
			consumer.ReceiveRtcpReceiverReport(report)
		}
		//fmt.Printf("get Consumer ReceiverReport ssrc %lu\n",packet.DestinationSSRC())
	case *rtcp.SourceDescription:
		//fmt.Printf("get Consumer SourceDescription %lu\n",packet.DestinationSSRC())
		mylog.Logger.Infof("get rtcp SourceDescription streamKey[%s] peerId[%s],string[%s]\n", this.listener.OnTransportGetRouterId(), this.Id,t.String())
	case *rtcp.TransportLayerNack:
		mylog.Logger.Infof("get rtcp TransportLayerNack streamKey[%s] peerId[%s] MediaSSRC[%v] string[%s]\n", this.listener.OnTransportGetRouterId(),this.Id,t.MediaSSRC,t.String())
		consumer := this.GetConsumerByMediaSsrc(t.MediaSSRC)
		if (nil == consumer) {
			mylog.Logger.Warnf(
				"no Consumer found for received %s nack packet [sender ssrc:%v, media ssrc:%v]",
				t.MediaSSRC, t.SenderSSRC)
			return
		}
		consumer.ReceiveNack(t)

	case *rtcp.RapidResynchronizationRequest:
		//fmt.Printf("get Consumer RapidResynchronizationRequest %lu\n",packet.DestinationSSRC())
		mylog.Logger.Infof("get rtcp RapidResynchronizationRequest streamKey[%s] peerId[%s] SenderSSRC[%v] MediaSSRC[%v] string[%s]\n", this.listener.OnTransportGetRouterId(),this.Id,t.SenderSSRC, t.MediaSSRC, t.String())
	case *rtcp.RawPacket:
		mylog.Logger.Infof("get rtcp streamKey[%s] peerId[%s] RawPacket  string[%s]\n", this.listener.OnTransportGetRouterId(),this.Id,t.String())
		//fmt.Printf("get Consumer RawPacket %lu\n",packet.DestinationSSRC())
	case *rtcp.PictureLossIndication:
		mylog.Logger.Infof("get rtcp PLI streamKey[%s] peerId[%s] received, requesting key frame for Consumer [sender ssrc:%v, media ssrc:%v]\n", this.listener.OnTransportGetRouterId(),this.Id,t.SenderSSRC, t.MediaSSRC)
		consumer := this.GetConsumerByMediaSsrc(t.MediaSSRC)
		if (nil == consumer) {
			mylog.Logger.Warnf(
				"no Consumer found for received %s Feedback packet [sender ssrc:%v, media ssrc:%v]",
				t.MediaSSRC, t.SenderSSRC)
			return
		}
		consumer.ReceiveKeyFrameRequestPLI(t)
	case *rtcp.SliceLossIndication:
		mylog.Logger.Infof("get rtcp SliceLossIndication streamKey[%s] peerId[%s] SenderSSRC[%v] MediaSSRC[%v] string[%s]\n", this.listener.OnTransportGetRouterId(),this.Id,t.SenderSSRC, t.MediaSSRC, t.String())
	case *rtcp.ReceiverEstimatedMaximumBitrate:
		mylog.Logger.Infof("get rtcp ReceiverEstimatedMaximumBitrate streamKey[%s] peerId[%s] SenderSSRC[%v]  string[%s]\n", this.listener.OnTransportGetRouterId(),this.Id,t.SenderSSRC, t.String())
	case *rtcp.Goodbye:
		mylog.Logger.Infof("get rtcp Goodbye streamKey[%s] peerId[%s] Sources[%v] Reason[%s] DestinationSSRC[%v]\n", this.listener.OnTransportGetRouterId(),this.Id,t.Sources, t.Reason, t.DestinationSSRC())
	default:
	}
	//fmt.Printf("get Consumer rtcp no type\n")
}

func (this *Transport) GetConsumerByMediaSsrc(ssrc uint32) consumer.InferfaceConsumer {
	//return this.mapSsrcConsumer[ssrc]
	v,ok := this.mapSsrcConsumer.Load(ssrc)
	if (false == ok){
		return nil
	}

	return v.(consumer.InferfaceConsumer)
}

func (this *Transport) OnConsumerKeyFrameRequested(consumer consumer.InferfaceConsumer, mappedSsrc uint32) {
	this.listener.OnTransportConsumerKeyFrameRequested(this, consumer, mappedSsrc)
}

func (this *Transport) OnProducerSendRtcpPacket(producer *producer.Producer, packet []rtcp.Packet) {
	this.listenerChild.SendRtcpPacket(packet)
}

func (this *Transport) OnProducerRtcpSenderReport(producer *producer.Producer, rtpStream *streamRecv.StreamRecv, first bool) {
	this.listener.OnTransportProducerRtcpSenderReport(this, producer, rtpStream, first)
}

func (this *Transport) OnProducerNeedWorstRemoteFractionLost(producer *producer.Producer, mappedSsrc uint32, worstRemoteFractionLost *uint8) {
	this.listener.OnTransportNeedWorstRemoteFractionLost(
		this, producer, mappedSsrc, worstRemoteFractionLost)
}

func (this *Transport) OnConsumerProducerClosed(consumer consumer.InferfaceConsumer) {
	delete(this.MapConsumers, consumer.ID())

	for _, ssrc := range consumer.GetMediaSsrcs() {
		//delete(this.mapSsrcConsumer, ssrc)
		this.mapSsrcConsumer.Delete(ssrc)
	}
	this.listener.OnTransportConsumerProducerClosed(this, consumer)
}

func (this *Transport)AddWaitGroup(i int) {
	this.wg.Add(i)
}

func (this *Transport)DoneWaitGroup() {
	this.wg.Done()
}

func (this *Transport) SetCloseFlag(flag bool){
	this.CloseFlag = flag
}

func (this *Transport) Close() {
	this.SetCloseFlag(true)
	this.Cancel()
	this.wg.Wait()
	mylog.Logger.Infof("RoomTransport streamKey[%s] peerId[%s] close Transport wait end",this.listener.OnTransportGetRouterId(),this.Id)
	if this.job != nil{
		this.job.Cancel()
	}
	this.clock.Stop()
	this.RtpListener = nil


	for _, producer := range this.MapProducers {
		this.listener.OnTransportProducerClosed(this, producer)
	}
	this.MapProducers = nil

	for _, consumer := range this.MapConsumers {
		this.listener.OnTransportConsumerClosed(this, consumer)
	}
	this.MapConsumers = nil

	this.mapSsrcConsumer.Clear()
	this.mapSsrcConsumer = nil

}

func (this *Transport) IsRouterClose() bool {
	return this.listener.OnTransportIsRouterClosed()

}