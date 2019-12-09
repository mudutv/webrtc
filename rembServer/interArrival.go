package rembServer

import (
	"github.com/pion/utils"
	"github.com/pion/webrtc/mylog"
)

const ReorderedResetThreshold = 3
const ArrivalTimeOffsetThresholdMs = 3000
const BurstDeltaThresholdMs = 5

type TimestampGroup struct {
	Size             uint
	FirstTimestamp   uint32
	Timestamp        uint32
	CompleteTimeMs   int64
	LastSystemTimeMs int64
}

func newTimestampGroup() TimestampGroup {
	node := TimestampGroup{}
	node.Size = 0
	node.FirstTimestamp = 0
	node.Timestamp = 0
	node.CompleteTimeMs = -1

	return node

}

func (this *TimestampGroup) IsFirstPacket() bool {
	return this.CompleteTimeMs == -1
}

type InterArrival struct {
	timestampGroupLengthTicks      uint32
	currentTimestampGroup          TimestampGroup
	prevTimestampGroup             TimestampGroup
	timestampToMsCoeff             float64
	burstGrouping                  bool
	numConsecutiveReorderedPackets int
}

func NewInterArrival(timestampGroupLengthTicks uint32, timestampToMsCoeff float64, enableBurstGrouping bool) InterArrival{
	node := InterArrival{}
	node.timestampGroupLengthTicks = timestampGroupLengthTicks
	node.currentTimestampGroup = newTimestampGroup()
	node.prevTimestampGroup = newTimestampGroup()
	node.timestampToMsCoeff = timestampToMsCoeff
	node.burstGrouping = enableBurstGrouping
	node.numConsecutiveReorderedPackets = 0
	return node

}

func (this *InterArrival) ComputeDeltas(timestamp uint32,
	arrivalTimeMs int64,
	systemTimeMs int64,
	packetSize uint,
	timestampDelta *uint32,
	arrivalTimeDeltaMs *int64,
	packetSizeDelta *int) bool {
	if (nil == timestampDelta || nil == arrivalTimeDeltaMs || nil == packetSizeDelta) {
		mylog.Logger.Errorf("debug wrong\n")
		return false
	}

	calculatedDeltas := false

	if (this.currentTimestampGroup.IsFirstPacket()) {
		// We don't have enough data to update the filter, so we store it until we
		// have two frames of data to process.
		this.currentTimestampGroup.Timestamp = timestamp
		this.currentTimestampGroup.FirstTimestamp = timestamp
	} else if (!this.PacketInOrder(timestamp)) {
		return false;
	} else if (this.NewTimestampGroup(arrivalTimeMs, timestamp)) {
		// First packet of a later frame, the previous frame sample is ready.
		if (this.prevTimestampGroup.CompleteTimeMs >= 0) {
			*timestampDelta =
				this.currentTimestampGroup.Timestamp - this.prevTimestampGroup.Timestamp;
			*arrivalTimeDeltaMs =
				this.currentTimestampGroup.CompleteTimeMs - this.prevTimestampGroup.CompleteTimeMs;

			// Check system time differences to see if we have an unproportional jump
			// in arrival time. In that case reset the inter-arrival computations.
			systemTimeDeltaMs :=
				this.currentTimestampGroup.LastSystemTimeMs - this.prevTimestampGroup.LastSystemTimeMs

			if (*arrivalTimeDeltaMs-systemTimeDeltaMs >= ArrivalTimeOffsetThresholdMs) {
				mylog.Logger.Infof("the arrival time clock offset has changed, resetting,[diff:%v ms]", *arrivalTimeDeltaMs-systemTimeDeltaMs)

				this.Reset()

				return false
			}

			if (*arrivalTimeDeltaMs < 0) {
				// The group of packets has been reordered since receiving its local
				// arrival timestamp.
				this.numConsecutiveReorderedPackets++
				if (this.numConsecutiveReorderedPackets >= ReorderedResetThreshold) {
					mylog.Logger.Info(
						"packets are being reordered on the path from the socket to the bandwidth estimator, ignoring this packet for bandwidth estimation, resetting")
					this.Reset()
				}

				return false;
			}

			this.numConsecutiveReorderedPackets = 0

			//if ()
			//MS_ASSERT(*arrivalTimeDeltaMs >= 0, "invalid arrivalTimeDeltaMs value");

			*packetSizeDelta = int(this.currentTimestampGroup.Size) - int(this.prevTimestampGroup.Size)
			calculatedDeltas = true;
		}

		this.prevTimestampGroup = this.currentTimestampGroup
		// The new timestamp is now the current frame.
		this.currentTimestampGroup.FirstTimestamp = timestamp
		this.currentTimestampGroup.Timestamp = timestamp
		this.currentTimestampGroup.Size = 0
	} else {
		this.currentTimestampGroup.Timestamp =
			utils.LatestTimestamp(this.currentTimestampGroup.Timestamp, timestamp)
	}

	this.currentTimestampGroup.Size += packetSize
	this.currentTimestampGroup.CompleteTimeMs = arrivalTimeMs
	this.currentTimestampGroup.LastSystemTimeMs = systemTimeMs

	return calculatedDeltas
}

func (this *InterArrival) PacketInOrder(timestamp uint32) bool {
	if (this.currentTimestampGroup.IsFirstPacket()) {
		return true
	}

	// Assume that a diff which is bigger than half the timestamp interval
	// (32 bits) must be due to reordering. This code is almost identical to
	// that in IsNewerTimestamp() in module_common_types.h.
	timestampDiff := timestamp - this.currentTimestampGroup.FirstTimestamp

	return timestampDiff < 0x80000000
}

func (this *InterArrival) NewTimestampGroup(arrivalTimeMs int64, timestamp uint32) bool {
	if (this.currentTimestampGroup.IsFirstPacket()) {
		return false
	}

	if (this.BelongsToBurst(arrivalTimeMs, timestamp)) {
		return false
	}

	timestampDiff := timestamp - this.currentTimestampGroup.FirstTimestamp

	return timestampDiff > this.timestampGroupLengthTicks
}

func (this *InterArrival) BelongsToBurst(arrivalTimeMs int64, timestamp uint32) bool {
	if (!this.burstGrouping) {
		return false
	}

	arrivalTimeDeltaMs := arrivalTimeMs - this.currentTimestampGroup.CompleteTimeMs
	timestampDiff := timestamp - this.currentTimestampGroup.Timestamp
	tsDeltaMs := int64(utils.LroundMy(this.timestampToMsCoeff*float64(timestampDiff) + 0.5))

	if (tsDeltaMs == 0) {
		return true
	}

	propagationDeltaMs := arrivalTimeDeltaMs - tsDeltaMs;
	return propagationDeltaMs < 0 && arrivalTimeDeltaMs <= BurstDeltaThresholdMs
}

func (this *InterArrival) Reset() {
	this.numConsecutiveReorderedPackets = 0
	this.currentTimestampGroup = newTimestampGroup()
	this.prevTimestampGroup = newTimestampGroup()
}
