package webrtc

import "sync/atomic"

type atomicBool struct {
	val int32
}

func (b *atomicBool) set(value bool) {
	var i int32
	if value {
		i = 1
	}

	atomic.StoreInt32(&(b.val), i)
}

func (b *atomicBool) get() bool {
	return atomic.LoadInt32(&(b.val)) != 0
}
