package keyframerequestmanager

import (
	"github.com/pion/alex023/clock"
	"github.com/pion/webrtc/mylog"
	"sync"
	"time"
)

const KeyFrameWaitTime = 1000

type PKFLister interface {
	OnKeyFrameRequestTimeout(keyFrameRequestInfo *PendingKeyFrameInfo)
}
type PendingKeyFrameInfo struct {
	SSRC           uint32
	RetryOnTimeout bool
	listener       PKFLister
	job            clock.Job
	clock          *clock.Clock
}

func NewPendingKeyFrameInfo(lister PKFLister, ssrc uint32, clock *clock.Clock, pwg *sync.WaitGroup) *PendingKeyFrameInfo {
	this := &PendingKeyFrameInfo{SSRC: ssrc, RetryOnTimeout: true, listener: lister}

	job, ok := clock.AddJobWithInterval(KeyFrameWaitTime*time.Millisecond, func() {
		pwg.Add(1)
		defer pwg.Done()
		this.listener.OnKeyFrameRequestTimeout(this)
	})
	if !ok {
		mylog.Logger.Errorf("NewPendingKeyFrameInfo  AddJobWithInterval fail ssrc [%v]\n", ssrc)
		return nil
	}
	this.clock = clock
	this.job = job
	return this
}

func (p *PendingKeyFrameInfo) Release() {
	if (nil != p.clock) {
		p.job.Cancel()
		p.clock = nil
	}

}

func (p *PendingKeyFrameInfo) GetSsrc() uint32 {
	return p.SSRC
}

func (p *PendingKeyFrameInfo) GetRetryOnTimeout() bool {
	return p.RetryOnTimeout
}

func (p *PendingKeyFrameInfo) SetRetryOnTimeout(notify bool) {
	p.RetryOnTimeout = notify
}

func (p *PendingKeyFrameInfo) Restart(pwg *sync.WaitGroup) {
	//p.clock.UpdateJobTimeout(p.job, KeyFrameWaitTime*time.Millisecond)
	p.job.Cancel()
	job, ok := p.clock.AddJobWithInterval(KeyFrameWaitTime*time.Millisecond, func() {
		pwg.Add(1)
		defer pwg.Done()
		p.listener.OnKeyFrameRequestTimeout(p)
	})
	if !ok {
		mylog.Logger.Errorf("Restart  AddJobWithInterval fail ssrc [%v]\n", p.SSRC)
		return
	}
	p.job = job
}

type KFRMLister interface {
	OnKeyFrameNeeded(KeyFrameRequestManager *KeyFrameRequestManager, ssrc uint32)
}

type KeyFrameRequestManager struct {
	MapSsrcPendingKeyFrameInfo map[uint32]*PendingKeyFrameInfo
	listener                   KFRMLister
	Clock                      *clock.Clock
	wg                         sync.WaitGroup
}

func NewKeyFrameRequestManager(lister KFRMLister) *KeyFrameRequestManager {
	return &KeyFrameRequestManager{MapSsrcPendingKeyFrameInfo:make(map[uint32]*PendingKeyFrameInfo), listener:lister, Clock:clock.NewClock()}
}

func (p *KeyFrameRequestManager) Release() {
	for _, v := range p.MapSsrcPendingKeyFrameInfo {
		v.Release()
	}
	p.wg.Wait()
	p.Clock.Stop()

}

func (p *KeyFrameRequestManager) KeyFrameNeeded(ssrc uint32) {
	v, ok := p.MapSsrcPendingKeyFrameInfo[ssrc]
	if (ok) {
		v.SetRetryOnTimeout(true)
		mylog.Logger.Infof("KeyFrameNeeded aready exit ssrc[%v] \n", ssrc)
		return
	}
	p.MapSsrcPendingKeyFrameInfo[ssrc] = NewPendingKeyFrameInfo(p, ssrc, p.Clock, &p.wg)
	p.listener.OnKeyFrameNeeded(p, ssrc)
	mylog.Logger.Infof("KeyFrameNeeded success ssrc[%v] \n", ssrc)
}

func (p *KeyFrameRequestManager) ForceKeyFrameNeeded(ssrc uint32) {
	v, ok := p.MapSsrcPendingKeyFrameInfo[ssrc]
	if (ok) {
		v.SetRetryOnTimeout(true)
		v.Restart(&p.wg)

	} else {
		p.MapSsrcPendingKeyFrameInfo[ssrc] = NewPendingKeyFrameInfo(p, ssrc, p.Clock,&p.wg)
	}

	p.listener.OnKeyFrameNeeded(p, ssrc)
}

func (p *KeyFrameRequestManager) KeyFrameReceived(ssrc uint32) {
	v, ok := p.MapSsrcPendingKeyFrameInfo[ssrc]
	if (!ok) {
		return

	}
	v.Release()
	delete(p.MapSsrcPendingKeyFrameInfo, ssrc)
}

func (p *KeyFrameRequestManager) OnKeyFrameRequestTimeout(pendingKeyFrameInfo *PendingKeyFrameInfo) {
	mylog.Logger.Infof("OnKeyFrameRequestTimeout  ssrc timeout [%v] \n", pendingKeyFrameInfo.SSRC)
	v, ok := p.MapSsrcPendingKeyFrameInfo[pendingKeyFrameInfo.SSRC]
	if (!ok) {
		mylog.Logger.Errorf("OnKeyFrameRequestTimeout find ssrc [%v] fail\n", pendingKeyFrameInfo.SSRC)
		return
	}

	if (!pendingKeyFrameInfo.GetRetryOnTimeout()) {
		mylog.Logger.Infof("OnKeyFrameRequestTimeout end  ssrc timeout [%v] \n", pendingKeyFrameInfo.SSRC)
		v.Release()
		delete(p.MapSsrcPendingKeyFrameInfo, pendingKeyFrameInfo.SSRC)
		return;
	}

	// Best effort in case the PLI/FIR was lost. Do not retry on timeout.
	pendingKeyFrameInfo.SetRetryOnTimeout(false);
	pendingKeyFrameInfo.Restart(&p.wg)
	p.listener.OnKeyFrameNeeded(p, pendingKeyFrameInfo.GetSsrc());
}
