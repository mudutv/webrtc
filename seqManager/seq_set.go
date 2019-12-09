package seqManager

import (
	"fmt"
	"sort"
	"github.com/pion/webrtc/mylog"
)

type SequenceSet []uint16

func NewSetList() SequenceSet {
	return SequenceSet{}
}


func (s *SequenceSet) PushLowerThan(InsertSeq uint16) bool {
	//插入切片里，按升序排序
	Len := len(*s)

	index := sort.Search(Len, func(i int) bool {return CompareSeqNumLowerThan((*s)[i], InsertSeq) > 0 })
	//fmt.Println("miaobinwei", index)
	if index > 0 {
		// Check if this is a duplicate
		//检查是否重复
		if (*s)[index-1] == InsertSeq {
			return false
		}

	}

	*s = append(*s, InsertSeq)
	//fmt.Println(s)
	if index == Len{
		return true
	}

	copy((*s)[index+1:], (*s)[index:])
	(*s)[index] = InsertSeq
	//fmt.Println("miaobinwei")
	return true
}


func (s *SequenceSet) Del(InsertSeq uint16) bool {
	//插入切片里，按升序排序
	Len := len(*s)
	if 0 == Len{
		return false
	}

	index := sort.Search(Len, func(i int) bool {return CompareSeqNumLowerThan((*s)[i], InsertSeq) > 0 })
	//fmt.Println("miaobinwei", index)
	if index > 0 {
		// Check if this is a duplicate
		//检查是否有相等的
		if (*s)[index-1] == InsertSeq {
			*s=append((*s)[:index - 1],(*s)[index:]...)
			return true
		}

	}

	return false
}

func (s *SequenceSet)Lower_bound(InsertSeq uint16) (delOut []uint16){
	index := sort.Search(len(*s), func(i int) bool { return CompareSeqNumLowerThan((*s)[i], InsertSeq) > 0 })
	if index > 0 {
		delOut = append(delOut, (*s)[:index]...)
		*s=append((*s)[:0],(*s)[index:]...)
		return
	}

	return
}

func (s *SequenceSet)Len() (int){
	return len(*s)
}

func (s *SequenceSet)GetBegin() (uint16, bool){
	if 0 == s.Len(){
		return 0, false
	}
	return (*s)[0], true
}

func (s *SequenceSet)Clear(){
	*s = (*s)[0:0]
}

func (s *SequenceSet)Clone() SequenceSet{
	v := make(SequenceSet,len(*s),len(*s))
	copy(v,*s)
	return v
}

func (s *SequenceSet) PrintSeq()  {
	fmt.Println("SeSet print ")
	for _,v := range *s{
		mylog.Logger.Infof(" %d ", v)
	}
}