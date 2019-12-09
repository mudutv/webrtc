package httpclient

import (
	"github.com/pion/webrtc/mylog"
	"github.com/recallsong/httpc"
)

//var baseUrl = utils.GetEnvWithDefault("HTTP_SERVER_URL", "https://198dev.myun.tv/mrtc-gateway")
var baseUrl string= "https://198dev.myun.tv/mrtc-gateway"

func OnAnswer(url string, peerId string, sdp string){
	body := make(map[string]interface{})
	body["cmd"] = "on_answer"
	body["id"] = peerId
	body["sdp"] = sdp

	if (0 == len(url)){
		url = baseUrl
	}

	var resp map[string]interface{}
	// 根据请求的Content-Type自动对数据进行转换
	err := httpc.New(url).Path("server-callback").
		ContentType(httpc.TypeApplicationJson).
		Body(body,httpc.TypeApplicationJson). // body转变为 {"name":"RecallSong","age":18}
		Post(&resp,httpc.TypeApplicationJson) // 根据响应中的Content-Type，将返回的数据解析到resp中

	mylog.Logger.Infof("OnAnswer http url[%s] peerId[%s] rsp   err[%v] resp[%v]",  url, peerId, err, resp)
}

func OnOffer(url string, peerId string, sdp string){
	body := make(map[string]interface{})
	body["cmd"] = "on_offer"
	body["id"] = peerId
	body["sdp"] = sdp

	if (0 == len(url)){
		url = baseUrl
	}

	var resp map[string]interface{}
	// 根据请求的Content-Type自动对数据进行转换
	err := httpc.New(url).Path("server-callback").
		ContentType(httpc.TypeApplicationJson).
		Body(body). // body转变为 {"name":"RecallSong","age":18}
		Post(&resp) // 根据响应中的Content-Type，将返回的数据解析到resp中

	mylog.Logger.Infof("OnOffer http url[%s] peerId[%s] rsp   err[%v] resp[%v]", url, peerId, err, resp)
}
