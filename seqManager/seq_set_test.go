package seqManager

import (
	"fmt"
	"testing"
)

func TestSequentialPush(t *testing.T) {
	ss := SequenceSet{}

	ss.PushLowerThan(1)
	ss.PushLowerThan(3)
	ss.PushLowerThan(2)
	ss.PushLowerThan(9)
	ss.PushLowerThan(5)
	ss.PushLowerThan(511)
	ss.PushLowerThan(8)

	fmt.Println(ss)

	fmt.Println(ss.Lower_bound(600))
	fmt.Println(ss)

	//ss.Del(2)
	//fmt.Println(ss)
	//ss.PushLowerThan(4)
	//ss.PushLowerThan(5)
	//fmt.Println(ss)
	//ss.Del(5)
	//fmt.Println(ss)
	//ss.PushLowerThan(1)
	//fmt.Println(ss)
	//ss.Del(1)
	//fmt.Println(ss)
	//ss.Del(1)
	//fmt.Println(ss)
	//ss.Del(3)
	//ss.Del(1)
	//fmt.Println(ss)
	//ss.Del(4)
	//fmt.Println(ss)
	//ss.PushLowerThan(1)
	//fmt.Println(ss)


	//t.Error(len(ss),ss[0], ss[1], ss[2], ss[3])


}