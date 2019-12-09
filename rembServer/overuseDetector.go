package rembServer

import (
	"github.com/pion/utils"
	"math"
)

const OverUsingTimeThreshold = 10
const MaxAdaptOffsetMs = 15.0
const MinNumDeltas = 60

type OveruseDetector struct {
	up                     float64
	down                   float64
	overusingTimeThreshold float64
	threshold              float64
	lastUpdateMs           int64
	prevOffset             float64
	timeOverUsing          float64
	overuseCounter         int
	hypothesis             int //BW_NORMAL
}

func NewOveruseDetector() OveruseDetector {
	node := OveruseDetector{}
	node.up = 0.0087
	node.down = 0.039
	node.overusingTimeThreshold = OverUsingTimeThreshold
	node.threshold = 12.5
	node.lastUpdateMs = -1
	node.prevOffset = 0.0
	node.timeOverUsing = -1
	node.overuseCounter = 0
	node.hypothesis = BW_NORMAL
	return node
}

func (this *OveruseDetector) State() int {
	return this.hypothesis
}

func (this *OveruseDetector) Detect(offset float64, tsDelta float64, numOfDeltas int, nowMs int64) int {
	if (numOfDeltas < 2) {
		return BW_NORMAL
	}

	t := float64(utils.MinMy(int64(numOfDeltas), MinNumDeltas)) * offset
	if (t > this.threshold) {
		if (this.timeOverUsing == -1) {
			// Initialize the timer. Assume that we've been
			// over-using half of the time since the previous
			// sample.
			this.timeOverUsing = tsDelta / 2
		} else
		{
			// Increment timer
			this.timeOverUsing += tsDelta
		}

		this.overuseCounter++

		if (this.timeOverUsing > this.overusingTimeThreshold && this.overuseCounter > 1) {
			if (offset >= this.prevOffset) {
				this.timeOverUsing = 0
				this.overuseCounter = 0
				this.hypothesis = BW_OVERUSING
			}
		}
	} else if (t < -this.threshold) {
		this.timeOverUsing = -1
		this.overuseCounter = 0
		this.hypothesis = BW_UNDERUSING
	} else
	{
		this.timeOverUsing = -1
		this.overuseCounter = 0
		this.hypothesis = BW_NORMAL
	}

	this.prevOffset = offset
	this.UpdateThreshold(t, nowMs)

	return this.hypothesis
}

func (this *OveruseDetector) UpdateThreshold(modifiedOffset float64, nowMs int64) {

	if (this.lastUpdateMs == -1) {
		this.lastUpdateMs = nowMs
	}

	if (math.Abs(modifiedOffset) > this.threshold+MaxAdaptOffsetMs) {
		// Avoid adapting the threshold to big latency spikes, caused e.g.,
		// by a sudden capacity drop.
		this.lastUpdateMs = nowMs

		return
	}

	k := float64(0)
	if math.Abs(modifiedOffset) < this.threshold {
		k = this.down
	} else {
		k = this.up
	}
	maxTimeDeltaMs := int64(100)
	timeDeltaMs := utils.MinMy(nowMs-this.lastUpdateMs, maxTimeDeltaMs)

	this.threshold += k * (math.Abs(modifiedOffset) - this.threshold) * float64(timeDeltaMs)

	minThreshold := float64(6.0)
	maxThreshold := float64(600.0)

	this.threshold = math.Min(math.Max(this.threshold, minThreshold), maxThreshold)
	this.lastUpdateMs = nowMs
}
