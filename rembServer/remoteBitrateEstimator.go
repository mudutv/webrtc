package rembServer

import (
	"github.com/pion/webrtc/deque"
)
var streamTimeOutMs int64 = 2000

type Listener interface {

	 OnRembServerAvailableBitrate(remoteBitrateEstimator *RemoteBitrateEstimatorAbsSendTime, ssrcs	*deque.Deque, availableBitrate uint32)
}

type CallStatsObserver struct {

}

type RemoteBitrateEstimator struct {
	CallStatsObserver
	AvailableBitrate uint32
	listener Listener
}

func (this *RemoteBitrateEstimator)GetAvailableBitrate() uint32{
	return this.AvailableBitrate
}