package rateCalculator

import (
	"github.com/pion/rtp"
	"github.com/pion/webrtc/uvtime"
	"github.com/pion/webrtc/mylog"
	"math"
	"sync"
)

const DefaultWindowSize  =  1000
const DefaultBpsScale  = 8000.0

type BufferItem struct {
	Count uint64
}
type RateCalculator struct {
	buffer  []BufferItem
	// Time (in milliseconds) for oldest item in the time window.
	 oldestTime uint64
	// Index for the oldest item in the time window.
	 oldestIndex uint64
	// Total count in the time window.
	 totalCount uint64
	// Window Size (in milliseconds).
	 windowSize uint64
	// Scale in which the rate is represented.
	 scale float32 
	// Total bytes transmitted.
	 bytes uint64

	 lock sync.RWMutex
}

func NewRateCalculator(windowsize uint64, scale float32) RateCalculator{
	if (0 == windowsize && scale == 0.0){
		windowsize = DefaultWindowSize
		scale = DefaultBpsScale
	}
	 v := RateCalculator{windowSize:windowsize, scale:scale}
	 v.Reset()
	 return v
}

func (p *RateCalculator)Reset(){
	now := uvtime.GettimeMs()
	p.ResetByTime(uint64(now))

}

func (p *RateCalculator)ResetByTime(now uint64){
	p.lock.Lock()
	defer p.lock.Unlock()
	p.buffer = make([]BufferItem, p.windowSize,p.windowSize)
	p.totalCount = 0
	p.oldestIndex = 0
	p.oldestTime = now - uint64(p.windowSize)

}

func (p *RateCalculator)ResetByTimeNoLock(now uint64){
	p.buffer = make([]BufferItem, p.windowSize,p.windowSize)
	p.totalCount = 0
	p.oldestIndex = 0
	p.oldestTime = now - uint64(p.windowSize)

}

func (p *RateCalculator)GetBytes() uint64{
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.bytes
}

func (p *RateCalculator)Update(size uint64, now uint64) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if (now < p.oldestTime){
		return
	}

	p.bytes += size

	p.RemoveOldData(now)

	offset := p.windowSize -1
	index := p.oldestIndex + offset

	if (index >= p.windowSize){
		index -= p.windowSize
	}

	p.buffer[index].Count += size
	p.totalCount += size

}

func (p *RateCalculator)GetRate(now uint64) uint32{
	p.lock.Lock()
	defer p.lock.Unlock()

	p.RemoveOldData(now)
	scale := p.scale / float32(p.windowSize)

	return uint32(math.Trunc(float64(p.totalCount) * float64(scale) + 0.5))
}

func (p *RateCalculator)RemoveOldData(now uint64) {
	var newOldestTime uint64 = now - uint64(p.windowSize)

	if (newOldestTime < p.oldestTime){
		mylog.Logger.Errorf("current time [%v] is older than a previous [%v]\n", newOldestTime,  p.oldestTime)
		return
	}

	// We are in the same time unit (ms) as the last entry.
	if (newOldestTime == p.oldestTime){
		return
	}

	if (newOldestTime > p.oldestTime + uint64(p.windowSize)){
		p.ResetByTimeNoLock(now)
		return
	}

	for p.oldestTime < newOldestTime {
		oldestItem := p.buffer[p.oldestIndex]
		p.totalCount -= oldestItem.Count
		p.buffer[p.oldestIndex] = BufferItem{}
		p.oldestIndex++

		if (p.oldestIndex >= p.windowSize){
			p.oldestIndex = 0
		}
		p.oldestTime++

	}

	p.oldestTime = newOldestTime
}


type RtpDataCounter struct {
	rate RateCalculator
	packets int
}

func NewRtpDataCounter() *RtpDataCounter{
	v := RtpDataCounter{}
	v.rate = NewRateCalculator(2500,DefaultBpsScale)
	//v.rate.windowSize = 2500
	//v.rate.scale = DefaultBpsScale
	return &v
}
func (p *RtpDataCounter)Update(packet *rtp.Packet){
	now := uvtime.GettimeMs()
	p.packets++
	p.rate.Update(uint64(packet.RawLen),uint64(now))
}

func (p *RtpDataCounter)GetBitrate(now uint64)uint32{
	return p.rate.GetRate(now)
}

func (p *RtpDataCounter)GetPacketCount()int{
	return p.packets
}

func (p *RtpDataCounter)GetBytes()uint64{
	return p.rate.GetBytes()
}