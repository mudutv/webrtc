package room

import (
	"github.com/pion/webrtc"
	"github.com/pion/webrtc/mapsync"
	"github.com/pion/webrtc/router"
)

type RoomList struct {

	//ListMap sync.Map
	ListMap *mapsync.MapSync
	Api     *webrtc.API
}

var RoomGroup *RoomList = &RoomList{ListMap:mapsync.NewMapSync()}

func init(){

	m := webrtc.MediaEngine{}

	codecH264 := webrtc.NewRTPH264Codec(webrtc.DefaultPayloadTypeH264, 90000)
	codecH264.RTCPFeedback = append(codecH264.RTCPFeedback, webrtc.RTCPFeedback{"goog-remb", ""})
	codecH264.RTCPFeedback = append(codecH264.RTCPFeedback, webrtc.RTCPFeedback{"nack", ""})
	codecH264.RTCPFeedback = append(codecH264.RTCPFeedback, webrtc.RTCPFeedback{"nack", "pli"})
	m.RegisterCodec(codecH264)

	codecVP8 := webrtc.NewRTPVP8Codec(webrtc.DefaultPayloadTypeVP8, 90000)
	codecVP8.RTCPFeedback = append(codecVP8.RTCPFeedback, webrtc.RTCPFeedback{"goog-remb", ""})
	codecVP8.RTCPFeedback = append(codecVP8.RTCPFeedback, webrtc.RTCPFeedback{"nack", ""})
	codecVP8.RTCPFeedback = append(codecVP8.RTCPFeedback, webrtc.RTCPFeedback{"nack", "pli"})
	m.RegisterCodec(codecVP8)

	codecOpus := webrtc.NewRTPOpusCodec(webrtc.DefaultPayloadTypeOpus, 48000)
	//codecOpus.RTCPFeedback = append(codecOpus.RTCPFeedback, webrtc.RTCPFeedback{"goog-remb", ""})
	//codecOpus.RTCPFeedback = append(codecOpus.RTCPFeedback, webrtc.RTCPFeedback{"nack", ""})
	//codecOpus.RTCPFeedback = append(codecOpus.RTCPFeedback, webrtc.RTCPFeedback{"nack", "pli"})
	m.RegisterCodec(codecOpus)

	//添加fec
	//mediaInfoVideo := v.GetMedia("video")
	//codecInfos := mediaInfoVideo.GetCodecs()
	//for _,codecInfo := range  codecInfos{
	//	fmt.Printf("=================\n")
	//	fmt.Printf("miaobinwei  rate [%d]\n", codecInfo.GetRate())
	//	fmt.Printf("miaobinwei [%s] type[%v]\n", codecInfo.GetCodec(), codecInfo.GetType())
	//	fmt.Printf("miaobinwei param[%v]\n", codecInfo.GetParams())
	//	fmt.Printf("miaobinwei GetRTX[%v]\n", codecInfo.GetRTX())
	//	for _,v := range  codecInfo.GetRTCPFeedbacks(){
	//		fmt.Printf("miaobinwei Getfb[%v]\n",*v)
	//	}
	//
	//	if ("red"== codecInfo.GetCodec()){
	//		fmt.Printf("111111miaobinwei  add red\n")
	//		codec := webrtc.NewRTPRedCodec(uint8(codecInfo.GetType()), uint32(codecInfo.GetRate()))
	//		m.RegisterCodec(codec)
	//	}
	//
	//	if ("ulpfec"== codecInfo.GetCodec()){
	//		fmt.Printf("111111miaobinwei  add ulpfec\n")
	//		codec := webrtc.NewRTPUlpFecCodec(uint8(codecInfo.GetType()), uint32(codecInfo.GetRate()))
	//		m.RegisterCodec(codec)
	//	}
	//}
	RoomGroup.Api = webrtc.NewAPI(webrtc.WithMediaEngine(m))
}


func (this *RoomList)Load(StreamKey string) *router.Router{
	v,ok := this.ListMap.Load(StreamKey)
	if ok{
		return v.(*router.Router)
	}

	return nil
}

func (this *RoomList)Store(StreamKey string, router *router.Router)bool{
	_,ok :=   this.ListMap.LoadOrStore(StreamKey, router)

	return ok
}

func (this *RoomList)GetNum() int{
	return  this.ListMap.Len()
}

func (this *RoomList)Delete(StreamKey string){
	this.ListMap.Delete(StreamKey)
}


func (this *RoomList)GetStatus(f func(key, value interface{}) bool){
	this.ListMap.Range(f)
}
