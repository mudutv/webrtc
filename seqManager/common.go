package seqManager

import "math"

//https://blog.csdn.net/bdss58/article/details/78388858
const UINT32_MIN uint32 = 0
const UINT32_MAX = ^uint32(0)

const INT_MAX = int(^uint(0) >> 1)
const INT_MIN = ^INT_MAX

type sequenceNumber struct {
	value   uint16
	isValid bool
}

func (s *sequenceNumber) setValue(value uint16) {
	s.value = value
	s.isValid = true
}

func (s *sequenceNumber) reset() {
	s.isValid = false
}

func seqNumDistance(left uint16, right uint16) uint16 {
	ileft := int(left)
	iright := int(right)
	idist := ileft - iright
	if idist < 0 {
		return uint16(-idist)
	}
	return uint16(idist)
}


//对比左边跟右边的差值
func CompareSeqNumLowerThan(left uint16, right uint16) int16 {
	distance := seqNumDistance(left, right)
	adjustedLeft := int(left)
	adjustedRight := int(right)
	if distance > 32767 {
		if left > 32768 {
			adjustedLeft -= 65536
		}
		if right > math.MaxInt16+1 {
			adjustedRight -= 65536
		}
	}
	return int16(adjustedLeft - adjustedRight)
}

func CompareTimeStampHigherThan(lhs uint32, rhs uint32) bool {
	return ((lhs > rhs) && (lhs - rhs <= UINT32_MAX / 2)) ||
		((rhs > lhs) && (rhs - lhs > UINT32_MAX / 2));
}
