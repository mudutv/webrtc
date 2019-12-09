package rtpListener

import (
	"github.com/pion/webrtc/mapsync"
	"github.com/pion/webrtc/mylog"
	"github.com/pion/webrtc/producer"
)

type RtpListener struct {
	//SSRCTable map[uint32]*producer.Producer
	SSRCTable *mapsync.MapSync  // map[uint32]*producer.Producer
	MidTable map[string]*producer.Producer
	RidTable map[string]*producer.Producer
}

func NewRtpListener() *RtpListener{
	node := RtpListener{}
	//node.SSRCTable = make(map[uint32]*producer.Producer)
	node.SSRCTable = mapsync.NewMapSync()
	node.MidTable = make(map[string]*producer.Producer)
	node.RidTable = make(map[string]*producer.Producer)

	return &node
}

func (this *RtpListener)AddProducer(producer *producer.Producer){
	for _,ssrc := range producer.Track.GetSSRCS(){
		//if _,ok :=this.SSRCTable[uint32(ssrc)]; ok{
		//	mylog.Logger.Errorf("AddProducer ssrc[%v] exit", ssrc)
		//}else {
		//	this.SSRCTable[uint32(ssrc)] = producer
		//	mylog.Logger.Errorf("AddProducer ssrc[%v] success", ssrc)
		//}

		if _,ok := this.SSRCTable.Load(uint32(ssrc));ok{
			mylog.Logger.Errorf(" RtpListener AddProducer ssrc[%v] exit", ssrc)
		}else{
			this.SSRCTable.Store(uint32(ssrc),producer)
			mylog.Logger.Errorf("RtpListener AddProducer ssrc[%v] success", ssrc)
		}
	}
}

func (this *RtpListener)RemoveProducer(producer *producer.Producer){
	//for k,v := range this.SSRCTable{
	//	if v == producer{
	//		delete(this.SSRCTable,k)
	//	}
	//}

	this.SSRCTable.Clear()
}

func (this *RtpListener)GetProducerbySSRC(ssrc uint32) *producer.Producer{
	//return this.SSRCTable[ssrc]
	if v,ok := this.SSRCTable.Load(ssrc); ok{
		return v.(*producer.Producer)
	}

	return nil
}