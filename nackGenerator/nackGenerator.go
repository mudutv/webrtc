package nackGenerator

import (
	"github.com/pion/alex023/clock"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/mylog"
	"github.com/pion/webrtc/seqManager"
	"github.com/pion/webrtc/uvtime"
	"sync"
	"time"
)

const MaxPacketAge = 5000
const MaxNackPackets = 1000
const DefaultRtt = 80
const MaxNackRetries = 8
const TimerInterval = 50

const (
	SEQ = iota
	TIME
)

type Listener interface {
	OnNackGeneratorNackRequired(nackBatch []uint16)
	OnNackGeneratorKeyFrameRequired(ssrc uint32)
}
type NackInfo struct {
	Seq        uint16
	SendAtSeq  uint16
	SentAtTime uint64
	Retries    uint8
}
type NackGenerator struct {
	Started bool
	LastSeq uint16 // Seq number of last valid packet.
	Rtt     uint64 // Round trip time (ms).

	NackList     seqManager.SequenceMap
	KeyFrameList seqManager.SequenceSet

	listener Listener

	clock *clock.Clock
	wg sync.WaitGroup
	//timer *time.Timer
}

func NewNackGenerator(listener Listener) *NackGenerator {
	p := new(NackGenerator)

	p.NackList = seqManager.NewMapList()
	p.KeyFrameList = seqManager.NewSetList()

	p.Rtt = DefaultRtt

	p.listener = listener

	//p.timer = nil
	p.clock = clock.NewClock()

	return p
}

func (this *NackGenerator) Close() {
	this.wg.Wait()
	if (nil != this.clock){
		this.clock.Stop()
	}
	this.KeyFrameList = nil
	//if (nil != this.timer){
	//	this.timer.Stop()
	//}




}


func (this *NackGenerator) CleanOldNackItems(InsertSeq uint16) {
	this.NackList.Lower_bound(InsertSeq - MaxPacketAge)
	this.KeyFrameList.Lower_bound(InsertSeq - MaxPacketAge)
}

func (this *NackGenerator) RemoveNackItemsUntilKeyFrame() bool {
	if 0 == this.KeyFrameList.Len() {
		return false
	}
	SecLen := this.NackList.Len()
	Bseq, _ := this.KeyFrameList.GetBegin()

	//mylog.Logger.Infof("11remve key KeyList Bseq %v", Bseq)
	//this.KeyFrameList.PrintSeq()
	//mylog.Logger.Infof("11remve map KeyList Bseq %v", Bseq)
	//this.NackList.PrintMap()

	this.NackList.Lower_bound(Bseq)
	this.KeyFrameList.Del(Bseq)

	numItemsRemoved := SecLen - this.NackList.Len()

	if (numItemsRemoved > 0) {
		mylog.Logger.Infof("removed %v old NACK items older than received key frame [seq:%v]\n",
			numItemsRemoved,
			Bseq)

	}

	//mylog.Logger.Infof("22remve key KeyList Bseq %v", Bseq)
	//this.KeyFrameList.PrintSeq()
	//mylog.Logger.Infof("22remve map KeyList Bseq %v", Bseq)
	//this.NackList.PrintMap()

	return true
}

func (this *NackGenerator) AddPacketsToNackList(seqStart uint16, seqEnd uint16, ssrc uint32) {
	numNewNacks := seqEnd - seqStart
	if ((uint16(this.NackList.Len()) + numNewNacks) > MaxNackPackets) {
		this.NackList.Clear()
		this.KeyFrameList.Clear()
		this.listener.OnNackGeneratorKeyFrameRequired(ssrc)
		mylog.Logger.Infof("AddPacketsToNackList tooMaxSeq seqEnd: %v, seqStart:%v\n",
			seqEnd,
			seqStart)

		return
	}

	//将缺失的seq加入到nackList
	for seq := seqStart; seq != seqEnd; seq++ {
		sendAtSeq := seq + 0;
		this.NackList.PushLowerThan(seq, &NackInfo{seq, sendAtSeq, 0, 0})
	}
	//this.NackList.DateList.PrintSeq()

}

func (this *NackGenerator) GetNackBatch(NackFilterType int) (nackBatch []uint16) {
	nowtime := uvtime.GettimeMs()

	for _, k := range this.NackList.GetDateListClone() {
		//infer := this.NackList.DateMap[k]
		infer,ok := this.NackList.Find(k)
		if (false == ok){
			continue
		}
		nackInfo := infer.(*NackInfo)
		seq := nackInfo.Seq
		if (SEQ == NackFilterType) && (0 == nackInfo.SentAtTime) && (seqManager.CompareSeqNumLowerThan(this.LastSeq, nackInfo.SendAtSeq) > 0) {
			nackInfo.Retries++
			nackInfo.SentAtTime = uint64(nowtime)
			mylog.Logger.Infof("sequence number [%v] retries [%d]\n", seq,nackInfo.Retries)
			if (nackInfo.Retries >= MaxNackRetries) {
				mylog.Logger.Infof("sequence number removed from the NACK list due to max retries [seq:%v]\n", seq)

				this.NackList.Del(k);
			} else
			{
				nackBatch = append(nackBatch, seq)
			}

			continue;
		}

		if (TIME == NackFilterType) && (int64(nackInfo.SentAtTime+this.Rtt) < nowtime) {
			nackInfo.Retries++
			nackInfo.SentAtTime = uint64(nowtime)
			mylog.Logger.Infof("sequence number [%v] retries [%d]\n", seq,nackInfo.Retries)
			if (nackInfo.Retries >= MaxNackRetries) {
				mylog.Logger.Infof("sequence number removed from the NACK list due to max retries [seq:%v]\n", seq)

				this.NackList.Del(k)
			} else
			{
				nackBatch = append(nackBatch, seq)
			}

			continue
		}
	}
	return
}

func (this *NackGenerator) ReceivePacket(packet *rtp.Packet) bool {
	isKeyFrame := packet.IsKeyFrame()
	seq := packet.SequenceNumber
	if (!this.Started) {
		this.LastSeq = seq;
		this.Started = true;
		if (isKeyFrame) {
			this.KeyFrameList.PushLowerThan(seq);
		}

		//this.timer = time.NewTimer(time.Millisecond * TimerInterval)
		//this.timer = time.NewTicker(time.Millisecond * TimerInterval)
		return false;
	}

	if seq == this.LastSeq {
		return false;
	}

	if (seqManager.CompareSeqNumLowerThan(this.LastSeq, seq) > 0) {
		_, ok := this.NackList.Find(seq)
		if ok {
			this.NackList.Del(seq)
			mylog.Logger.Infof("NACKed packet received [ssrc:%v, seq:%v, lastSeq %v, pt %v]\n",
				packet.SSRC, seq, this.LastSeq,packet.PayloadType)
			return true
		}

		mylog.Logger.Infof("ignoring old packet not present in the NACK list [ssrc:%lu, seq:%u, lastSeq, %u]\n",
			packet.SSRC, seq, this.LastSeq)

		return false

	}

	this.CleanOldNackItems(seq)
	if (isKeyFrame) {
		mylog.Logger.Infof("NackGenerator recv keyframe [ssrc:%lu, seq:%v, lastSeq, %v]\n",
			packet.SSRC, seq, this.LastSeq)
		this.RemoveNackItemsUntilKeyFrame()
		this.KeyFrameList.PushLowerThan(seq)
		//this.KeyFrameList.PrintSeq()
	}

	//mylog.Logger.Errorf("seq[%v] this.LastSeq[%v]\n",seq, this.LastSeq )
	if (seq == this.LastSeq+1) {
		this.LastSeq++;
		return false;
	}

	this.AddPacketsToNackList(this.LastSeq+1, seq, packet.SSRC);
	if (!this.Started) {
		return false;
	}
	mylog.Logger.Infof("Add NACK buf LastSeq[%v] seq[%v]",this.LastSeq, seq)
	this.NackList.PrintMap()

	this.LastSeq = seq;

	// Check if there are any nacks that are waiting for this seq number.
	nackBatch := this.GetNackBatch(SEQ)

	//发送nack
	if (len(nackBatch) > 0) {
		mylog.Logger.Infof("NACK SEQ ssrc[%v]",packet.SSRC)
		this.listener.OnNackGeneratorNackRequired(nackBatch)
	}

	//if (nil != this.timer){
	//	select {
	//	case <-this.timer.C:
	//		nackBatch := this.GetNackBatch(TIME)
	//		if (len(nackBatch) > 0) {
	//			mylog.Logger.Infof("TIME")
	//			this.listener.OnNackGeneratorNackRequired(nackBatch,packet.SSRC)
	//		}
	//		this.timer = nil
	//	default:
	//
	//	}
	//}else {
	//	if this.NackList.Len() > 0{
	//		this.timer = time.NewTimer(time.Millisecond * TimerInterval)
	//	}
	//}
	this.MayRunTimer()

	return false
}

func (this *NackGenerator) OnTimer() {
	this.wg.Add(1)
	defer this.wg.Done()
	mylog.Logger.Infof("NACK TIME Begin")
	nackBatch := this.GetNackBatch(TIME)

	if (len(nackBatch) > 0) {
		mylog.Logger.Infof("NACK TIME")
		this.listener.OnNackGeneratorNackRequired(nackBatch)
	}

	this.MayRunTimer()
}

func (this *NackGenerator) MayRunTimer() {
	if (this.NackList.Len() > 0) {
		_, ok := this.clock.AddJobWithInterval(time.Millisecond*TimerInterval, this.OnTimer)
		if !ok {
			mylog.Logger.Errorf("nack  MayRunTimer fail \n")
			return
		}
	}

}


func (this *NackGenerator) Reset() {
	this.Started = false;
	this.LastSeq = 0;
	//this.Rtt     = 0;

	this.NackList.Clear()
	this.KeyFrameList.Clear()

	//if (nil != this.timer){
	//	this.timer.Stop()
	//	this.timer = nil
	//}

	if (nil != this.clock){
		this.clock.Reset()
	}

}
