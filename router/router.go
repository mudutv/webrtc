package router

import (
	"github.com/pion/rtp"
	"github.com/pion/webrtc"
	"github.com/pion/webrtc/consumer"
	"github.com/pion/webrtc/mapsync"
	"github.com/pion/webrtc/mylog"
	"github.com/pion/webrtc/producer"
	"github.com/pion/webrtc/streamRecv"
	"github.com/pion/webrtc/transbase"
)

type closeStruct struct {
	streamKey string
	peerId string
}
type Router struct {
	Id                   string
	//MapProducers         map[string]*producer.Producer //id
	MapProducers         *mapsync.MapSync //map[string]*producer.Producer //id

	//MapProducerConsumers map[*producer.Producer][]consumer.InferfaceConsumer
	MapProducerConsumers *mapsync.MapSync //map[*producer.Producer][]consumer.InferfaceConsumer

	//MapWebrtcTransports  map[string]*transbase.WebrtcTransport
	MapWebrtcTransports  *mapsync.MapSync//map[string]*transbase.WebrtcTransport

	//mapConsumerProducer  map[consumer.InferfaceConsumer]*producer.Producer
	mapConsumerProducer  *mapsync.MapSync//map[consumer.InferfaceConsumer]*producer.Producer
	CloseFlag bool
	closeChan chan closeStruct
}

func NewRouter(id string) *Router {
	node := &Router{}

	node.Id = id
	//node.MapProducers = make(map[string]*producer.Producer)
	node.MapProducers = mapsync.NewMapSync()
	//node.MapProducerConsumers = make(map[*producer.Producer][]consumer.InferfaceConsumer)
	node.MapProducerConsumers = mapsync.NewMapSync()
	//node.MapWebrtcTransports = make(map[string]*transbase.WebrtcTransport)
	node.MapWebrtcTransports = mapsync.NewMapSync()
	//node.mapConsumerProducer = make(map[consumer.InferfaceConsumer]*producer.Producer)
	node.mapConsumerProducer = mapsync.NewMapSync()
	node.CloseFlag = false
	node.closeChan = make(chan closeStruct,300)
	node.RunCloseing()
	return node
}

func (this *Router)RunCloseing(){
	go func() {
		mylog.Logger.Infof("RunCloseing streamKey[%s] begin", this.Id)
		for date := range this.closeChan{
			tp := this.GetWebrtcTransportById(date.peerId)
			if (nil == tp) {
				mylog.Logger.Errorf("ClosePeer get Transport in RunCloseing fail streamKey[%s] peerId[%s]", date.streamKey, date.peerId)
				return
			}

			if transbase.GET_TYPE == tp.PCType{
				mylog.Logger.Errorf("ClosePeer GET_TYPE streamKey[%s] peerId[%s]", date.streamKey, date.peerId)
				this.TransportClose(tp)
			}else{
				mylog.Logger.Errorf("ClosePeer push streamKey[%s] peerId[%s]", date.streamKey, date.peerId)
				this.SetCloseFlag(true)
				this.Close()

			}
		}
		mylog.Logger.Infof("RunCloseing streamKey[%s] end", this.Id)
	}()

}


func (this *Router)CloseWaiting(streamKey string, peerId string){
	if this.closeChan != nil && (false == this.OnTransportIsRouterClosed()){
		date := closeStruct{streamKey:streamKey,peerId:peerId}
		this.closeChan <- date
	}
}

func (this *Router) CreateWebrtcTransport(transportId string, pctype int) *transbase.WebrtcTransport {
	if _, ok := this.MapWebrtcTransports.Load(transportId); ok {
		mylog.Logger.Errorf("CreateWebrtcTransport exit transportId[%s]", transportId)
		return nil
	}

	tp := transbase.NewWebrtcTransport(transportId, this,pctype)
	if (nil == tp) {
		mylog.Logger.Errorf("CreateWebrtcTransport fail transportId[%s]", transportId)
		return nil
	}

	//this.MapWebrtcTransports.Store(transportId, tp)
	this.StoreWebrtcTransports(tp)

	return tp
}

func (this *Router) CreateWebrtcTransportNoStore(transportId string, pctype int) *transbase.WebrtcTransport {
	if _, ok := this.MapWebrtcTransports.Load(transportId); ok {
		mylog.Logger.Errorf("CreateWebrtcTransportNoStore exit transportId[%s]", transportId)
		return nil
	}

	tp := transbase.NewWebrtcTransport(transportId, this,pctype)
	if (nil == tp) {
		mylog.Logger.Errorf("CreateWebrtcTransportNoStore fail transportId[%s]", transportId)
		return nil
	}

	//mylog.Logger.Infof("RoomTransport streamKey[%s] peerId[%s] add success now num[%d]", this.Id,transportId,this.MapWebrtcTransports.Len())

	return tp
}

func (this *Router) StoreWebrtcTransports(tp *transbase.WebrtcTransport) {
	this.MapWebrtcTransports.Store(tp.Id, tp)
	mylog.Logger.Infof("RoomTransport streamKey[%s] peerId[%s] add success now num[%d]", this.Id,tp.Id,this.MapWebrtcTransports.Len())
	return
}



func (this *Router) GetWebrtcTransportById(transportId string) *transbase.WebrtcTransport {
	tp, ok := this.MapWebrtcTransports.Load(transportId)
	if ok {
		return tp.(*transbase.WebrtcTransport)
	}

	return nil
}

func (this *Router) SetCloseFlag(flag bool) {
	this.CloseFlag = flag
}
//对应CloseProducersAndConsumers
func (this *Router) Close() bool {
	this.CloseFlag = true
	//for _,tp := range this.MapWebrtcTransports{
	//	mylog.Logger.Errorf("RoomTransport streamKey[%s] peerId[%s] close remove WebrtcTransportsNum[%d]",this.Id,  tp.Id,len(this.MapWebrtcTransports))
	//	delete(this.MapWebrtcTransports, tp.Id)
	//	tp.Close()
	//}

	mylog.Logger.Errorf("RoomTransport streamKey[%s] Router close remove begin now WebrtcTransportsNum[%d]",this.Id, this.MapWebrtcTransports.Len())

	tpmap := make([]*transbase.WebrtcTransport,0,100)
	this.MapWebrtcTransports.Range(func(key, value interface{}) bool {
		tp := value.(*transbase.WebrtcTransport)
		closeflag := tp.IsClose()
		mylog.Logger.Errorf("RoomTransport streamKey[%s] peerId[%s] closeflag[%v] close remove",this.Id,  tp.Id, closeflag)
		if (closeflag == false){
			tp.SetCloseFlag(true)
			//this.MapWebrtcTransports.Delete(key)
			tpmap = append(tpmap,tp)
		}

		//tp.Close()
		return true
	})

	mylog.Logger.Errorf("RoomTransport streamKey[%s] Router close remove get live num [%d]",this.Id, len(tpmap))
	for _,tp := range tpmap{
		//放到这里避免range中堵塞，线程切换
		this.MapWebrtcTransports.Delete(tp.Id)
		tp.Close()
	}

	//this.MapWebrtcTransports.Range(func(key, value interface{}) bool {
	//	tp := value.(*transbase.WebrtcTransport)
	//	mylog.Logger.Errorf("RoomTransport streamKey[%s] peerId[%s] close remove",this.Id,  tp.Id)
	//	this.MapWebrtcTransports.Delete(key)
	//	tp.Close()
	//	return true
	//})
	mylog.Logger.Errorf("RoomTransport streamKey[%s] Router close remove end WebrtcTransportsNum[%d]",this.Id, this.MapWebrtcTransports.Len())
	this.MapWebrtcTransports = nil

	if (this.MapProducerConsumers.Len() > 0 || this.MapProducers.Len() > 0 || this.mapConsumerProducer.Len() > 0){
		mylog.Logger.Errorf("RoomTransport [%s] close but warn MapProducerConsumers[%d] MapProducers[%d] mapConsumerProducer[%d] ",this.Id, this.MapProducerConsumers.Len(),this.MapProducers.Len(),this.mapConsumerProducer.Len())
	}

	//this.MapProducerConsumers.Clear()
	this.MapProducerConsumers = nil
	//this.MapProducers.Clear()
	this.MapProducers = nil
	//this.mapConsumerProducer.Clear()
	this.mapConsumerProducer = nil
	close(this.closeChan)


	return true
}


//对应CloseProducersAndConsumers
func (this *Router) TransportClose(tp *transbase.WebrtcTransport) bool {
	//tp, ok := this.MapWebrtcTransports[transportId]
	//tp := this.GetWebrtcTransportById(transportId)
	//if nil == tp {
	//	mylog.Logger.Errorf("RoomTransport streamKey[%s] peerId[%s]  TransportClose remove fail WebrtcTransportsNum[%d]", this.Id, transportId, this.MapWebrtcTransports.Len())
	//	return false
	//}

	if true == tp.IsClose(){
		mylog.Logger.Errorf("RoomTransport streamKey[%s] peerId[%s]  Transport Close remove but is closeing",this.Id,  tp.Id)
		return false
	}
	tp.SetCloseFlag(true) //因为下面可能造成堵塞，线程抢占，所以直接先置关闭标志位
	mylog.Logger.Errorf("RoomTransport streamKey[%s] peerId[%s]  Transport Close remove now WebrtcTransportsNum[%d]",this.Id,  tp.Id, this.MapWebrtcTransports.Len())
	this.MapWebrtcTransports.Delete(tp.Id)
	tp.Close()

	return true
}

func (this *Router) OnTransportNewProducer(transport *transbase.Transport, producer *producer.Producer) {
	//_, ok := this.MapProducerConsumers[producer]
	_, ok := this.MapProducerConsumers.Load(producer)
	if (true == ok) {
		mylog.Logger.Error("Producer already present in mapProducerConsumers")
		return
	}

	//_, ok = this.MapProducers[producer.Id]
	_, ok = this.MapProducers.Load(producer.Id)
	if (true == ok) {
		mylog.Logger.Errorf("Producer already present in mapProducers [producerId:%s]", producer.Id)

	}

	//this.MapProducers[producer.Id] = producer
	this.MapProducers.Store(producer.Id,producer)
	//this.MapProducerConsumers[producer] = make([]consumer.InferfaceConsumer, 0, 3)
	this.MapProducerConsumers.Store(producer, make([]consumer.InferfaceConsumer, 0, 3))
	//this->mapProducerRtpObservers[producer];
}

func (this *Router) OnTransportNewConsumer(transport *transbase.Transport, consumerNew consumer.InferfaceConsumer, producerId string) {
	//producer, ok := this.MapProducers[producerId]
	v, ok := this.MapProducers.Load(producerId)
	if (true != ok) {
		mylog.Logger.Errorf("Producer not found [producerId:%s]", producerId)
		return

	}
	producer := v.(*producer.Producer)

	//mapProducerConsumersIt, ok := this.MapProducerConsumers[producer]
	mapProducerConsumersIt, ok := this.MapProducerConsumers.Load(producer)
	if (true != ok) {
		mylog.Logger.Errorf("Producer not present in mapProducerConsumers [producerId:%s]", producerId)
		return
	}
	mapProducerConsumersItNew := make([]consumer.InferfaceConsumer, 0, 3)
	mapProducerConsumersItNew = append(mapProducerConsumersItNew, mapProducerConsumersIt.([]consumer.InferfaceConsumer)...)
	mapProducerConsumersItNew = append(mapProducerConsumersItNew, consumerNew)
	//this.MapProducerConsumers[producer] = append(mapProducerConsumersIt, consumer)
	this.MapProducerConsumers.Store(producer, mapProducerConsumersItNew)
	//this.mapConsumerProducer[consumerNew] = producer
	this.mapConsumerProducer.Store(consumerNew,producer)

	for _, v := range producer.GetRtpStreams() {
		consumerNew.ProducerRtpStream(v)
	}

}

func (this *Router) OnTransportProducerRtpPacketReceived(transport *transbase.Transport, producer *producer.Producer, packet *rtp.Packet) {
	//consumers, ok := this.MapProducerConsumers[producer]
	consumers, ok := this.MapProducerConsumers.Load(producer)
	if (ok == false) {
		mylog.Logger.Errorf("OnTransportProducerRtpPacketReceived cant find produce[%s]->consumers", producer.Id)
		return
	}

	for _, consumer := range consumers.([]consumer.InferfaceConsumer) {
		consumer.SendRtpPacket(packet)
	}

	//auto it = this->mapProducerRtpObservers.find(producer);
}

func (this *Router) OnTransportConsumerKeyFrameRequested(transport *transbase.Transport, consumer consumer.InferfaceConsumer, mappedSsrc uint32) {
	//producer := this.mapConsumerProducer[consumer]
	v,ok := this.mapConsumerProducer.Load(consumer)
	if false == ok{
		return
	}

	producer := v.(*producer.Producer)
	producer.RequestKeyFrame(mappedSsrc)

}

func (this *Router) OnTransportProducerRtcpSenderReport(tansport *transbase.Transport, producer *producer.Producer, rtpStream *streamRecv.StreamRecv, first bool) {
	//consumers := this.MapProducerConsumers[producer]
	consumers,ok := this.MapProducerConsumers.Load(producer)
	if (ok == false) {
		mylog.Logger.Errorf("OnTransportProducerRtcpSenderReport cant find producer[%s]->consumers", producer.Id)
		return
	}

	for _, consumer := range consumers.([]consumer.InferfaceConsumer) {
		consumer.ProducerRtcpSenderReport(rtpStream, first)
	}
}

func (this *Router) OnTransportNeedWorstRemoteFractionLost(tansport *transbase.Transport, producer *producer.Producer, mappedSsrc uint32, worstRemoteFractionLost *uint8) {
	//consumers := this.MapProducerConsumers[producer]
	consumers,ok := this.MapProducerConsumers.Load(producer)
	if (ok == false) {
		mylog.Logger.Errorf("OnTransportNeedWorstRemoteFractionLost cant find producer[%s]->consumers", producer.Id)
		return
	}

	for _, consumer := range consumers.([]consumer.InferfaceConsumer) {
		consumer.NeedWorstRemoteFractionLost(mappedSsrc, worstRemoteFractionLost)
	}
}

func (this *Router) OnTransportProducerClosed(tansport *transbase.Transport, producer *producer.Producer) {
	//mapProducerConsumersIt := this.MapProducerConsumers[producer]
	mapProducerConsumersIt,ok := this.MapProducerConsumers.Load(producer)
	if (ok == false) {
		mylog.Logger.Errorf("OnTransportProducerClosed cant find producer[%s]->consumers", producer.Id)
		return
	}

	this.MapProducerConsumers.Delete(producer)
	for _, consumer := range mapProducerConsumersIt.([]consumer.InferfaceConsumer) {
		consumer.ProducerClosed()
	}

	//delete(this.MapProducerConsumers, producer)

	//delete(this.MapProducers, producer.Id)
	this.MapProducers.Delete(producer.Id)
	producer.Close()

}


func (this *Router) OnTransportConsumerProducerClosed(transport *transbase.Transport, consumer consumer.InferfaceConsumer) {
	//delete(this.mapConsumerProducer, consumer)
	this.mapConsumerProducer.Delete(consumer)
	consumer.Close()
}

func (this *Router)OnTransportConsumerClosed(tansport *transbase.Transport, consumerNew consumer.InferfaceConsumer){
	//Producer,ok := this.mapConsumerProducer[consumerNew]
	v,ok := this.mapConsumerProducer.Load(consumerNew)
	if (false == ok){
		mylog.Logger.Errorf("[OnTransportConsumerClosed]ConsumerId[%s] not present in mapConsumerProducer", consumerNew.ID())
		//delete(this.mapConsumerProducer, consumerNew)
		//this.mapConsumerProducer.Delete(consumerNew)
		return
	}

	Producer := v.(*producer.Producer)
	info,ok := this.MapProducerConsumers.Load(Producer)
	if (ok == false) {
		mylog.Logger.Errorf("OnTransportConsumerClosed cant find producer[%s]->consumers", Producer.Id)
		this.mapConsumerProducer.Delete(consumerNew)
		return
	}
	consumers := info.([]consumer.InferfaceConsumer)
	for index,v := range consumers{
		if v == consumerNew{
			consumers = append(consumers[:index],consumers[index + 1:]...)
			break
		}

	}
	//this.MapProducerConsumers[Producer] = consumers
	this.MapProducerConsumers.Store(Producer,consumers)
	//delete(this.mapConsumerProducer, consumer)
	//delete(this.mapConsumerProducer, consumerNew)
	this.mapConsumerProducer.Delete(consumerNew)
	consumerNew.Close()

}


func (this *Router)OnTransportIsRouterClosed() bool{
	return this.CloseFlag
}

func (this *Router)IsProducerSupportVideoAudio()(bool,  bool)  {
	videoFlag := false
	audioFlag := false
	this.MapProducers.Range(func(key, value interface{}) bool {
		producer := value.(*producer.Producer)
		if (webrtc.RTPCodecTypeVideo == producer.Kind){
			videoFlag = true
		}else if  webrtc.RTPCodecTypeAudio == producer.Kind{
			audioFlag = true
		}

		return true
	})

	return  videoFlag,audioFlag
}

func (this *Router)GetProducerVideoAbsTime() uint8 {
	var absSendTime uint8
	this.MapProducers.Range(func(key, value interface{}) bool {
		producer := value.(*producer.Producer)
		if webrtc.RTPCodecTypeVideo == producer.Kind {
			absSendTime = producer.RtpHeaderExtensionIds.AbsSendTime
			return false
		}
		return true
	})



	return  absSendTime
}

func (this *Router)OnTransportGetRouterId() string{
	return this.Id
}

func (this *Router)WebrtcTransportsRange(f func(key, value interface{}) bool){
	this.MapWebrtcTransports.Range(f)
}