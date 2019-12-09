package seqManager

import (
	"github.com/pion/webrtc/mylog"
	"sync"
)

type SequenceMap struct {
	dateMap  map[uint16]interface{}
	dateList SequenceSet
	rwmutex  sync.RWMutex
}

func NewMapList() SequenceMap {
	return SequenceMap{
		dateMap:  make(map[uint16]interface{}),
		dateList: NewSetList(),
	}
}

func (s *SequenceMap) PushLowerThan(InsertSeq uint16, Date interface{}) bool {
	s.rwmutex.Lock()
	defer 	s.rwmutex.Unlock()
	s.dateList.PushLowerThan(InsertSeq)
	s.dateMap[InsertSeq] = Date


	return true
}

func (s *SequenceMap) Find(InsertSeq uint16) (interface{}, bool) {
	s.rwmutex.RLock()
	defer s.rwmutex.RUnlock()
	v, ok := s.dateMap[InsertSeq]

	return v, ok
}

func (s *SequenceMap) Del(InsertSeq uint16) (bool) {
	s.rwmutex.Lock()
	defer s.rwmutex.Unlock()
	delete(s.dateMap, InsertSeq)
	return s.dateList.Del(InsertSeq)
}

func (s *SequenceMap) Lower_bound(InsertSeq uint16) bool {
	s.rwmutex.Lock()
	defer s.rwmutex.Unlock()
	delOut := s.dateList.Lower_bound(InsertSeq)
	if len(delOut) <= 0 {
		return false
	}

	for _, v := range delOut {
		delete(s.dateMap, v)
	}

	return true
}

func (s *SequenceMap) Len() (int) {
	s.rwmutex.RLock()
	defer s.rwmutex.RUnlock()
	return len((*s).dateMap)
}

func (s *SequenceMap) Clear() {
	s.rwmutex.Lock()
	defer s.rwmutex.Unlock()
	s.dateList.Clear()
	s.dateMap = make(map[uint16]interface{})


}

func (s *SequenceMap) GetDateListClone() SequenceSet{
	s.rwmutex.RLock()
	defer s.rwmutex.RUnlock()
	return s.dateList.Clone()
}

func (s *SequenceMap) PrintMap() {
	s.rwmutex.RLock()
	defer s.rwmutex.RUnlock()
	mylog.Logger.Infof("SeMap print set %v\n", s.dateList.PrintSeq)
	mylog.Logger.Infof("SeMap print  map %v\n", s.dateMap)

}
