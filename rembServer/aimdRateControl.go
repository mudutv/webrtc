package rembServer

import (
	"github.com/pion/utils"
	"math"
	"github.com/pion/webrtc/mylog"
)

const MaxFeedbackIntervalMs  = 1000
const DefaultRttMs  = 200
const MinBitrateBps  = 10000

const (
	RC_HOLD = iota
	RC_INCREASE
	RC_DECREASE
)

const (
	RC_NEAR_MAX =iota
	RC_ABOVE_MAX
	RC_MAX_UNKNOWN
)

type AimdRateControl struct {
	minConfiguredBitrateBps uint32
	maxConfiguredBitrateBps uint32
	currentBitrateBps uint32
	avgMaxBitrateKbps float32
	varMaxBitrateKbps float32
	rateControlState int //RC_HOLD
	rateControlRegion int //RC_NEAR_MAX
	timeLastBitrateChange int64
	currentInput RateControlInput
	updated bool
	timeFirstIncomingEstimate int64
	bitrateIsInitialized bool
	beta float32
	rtt int64
	lastDecrease int
}

func NewAimdRateControl() AimdRateControl{
	node := AimdRateControl{}
	node.minConfiguredBitrateBps = MinBitrateBps
	node.maxConfiguredBitrateBps = 30000000
	node.currentBitrateBps = node.maxConfiguredBitrateBps
	node.avgMaxBitrateKbps = -1.0
	node.varMaxBitrateKbps = 0.4
	node.rateControlState = RC_HOLD
	node.rateControlRegion = RC_MAX_UNKNOWN
	node.timeLastBitrateChange = -1
	node.currentInput = NewRateControlInput(BW_NORMAL, 0, 1.0)
	node.updated = false
	node.timeFirstIncomingEstimate = -1
	node.bitrateIsInitialized = false
	node.beta = 0.85
	node.rtt = DefaultRttMs
	node.lastDecrease = 0
	return node
}

func (this *AimdRateControl)SetStartBitrate(startBitrateBps int)  {
	this.currentBitrateBps    = uint32(startBitrateBps)
	this.bitrateIsInitialized = true
}

func (this *AimdRateControl)SetMinBitrate(minBitrateBps int)  {
	this.minConfiguredBitrateBps = uint32(minBitrateBps)

	if (uint32(minBitrateBps) > this.currentBitrateBps){
		this.currentBitrateBps = uint32(minBitrateBps)
	}
}

func (this *AimdRateControl)ValidEstimate()  bool{
	return this.bitrateIsInitialized
}

func (this *AimdRateControl)LatestEstimate()  uint32{
	return this.currentBitrateBps
}

func (this *AimdRateControl)UpdateBandwidthEstimate(nowMs int64)  uint32{
	this.currentBitrateBps =
		this.ChangeBitrate(this.currentBitrateBps, this.currentInput.IncomingBitrate, nowMs)

	return this.currentBitrateBps
}

func (this *AimdRateControl)SetRtt(rtt int64)  {
	this.rtt = rtt
}

func (this *AimdRateControl)SetEstimate(bitrateBps int, nowMs int64)  {
	this.updated               = true
	this.bitrateIsInitialized  = true
	this.currentBitrateBps     = this.ClampBitrate(uint32(bitrateBps), uint32(bitrateBps))
	this.timeLastBitrateChange = nowMs
}

func (this *AimdRateControl)GetLastBitrateDecreaseBps()  int{
	return this.lastDecrease
}

func (this *AimdRateControl)AdditiveRateIncrease(nowMs int64, lastMs int64)  uint32{
	return uint32((nowMs - lastMs) * int64(this.GetNearMaxIncreaseRateBps()) / 1000)
}

func (this *AimdRateControl)ChangeRegion(region int)  {
	this.rateControlRegion = region
}

func (this *AimdRateControl)ChangeStateBase(newState int)  {
	this.rateControlState = newState
}


func (this *AimdRateControl)GetFeedbackInterval()  int64{
	const RtcpSize = 80
	const minFeedbackIntervalMs  = 200
	interval := int64( utils.LroundMy((RtcpSize * 8.0 * 1000.0) / (0.05 * float64(this.currentBitrateBps)) + 0.5) )


	return utils.MinMy(utils.MaxMy(interval, minFeedbackIntervalMs), MaxFeedbackIntervalMs)
}

func (this *AimdRateControl) TimeToReduceFurther(timeNow int64, incomingBitrateBps uint32) bool {
	bitrateReductionInterval := utils.MaxMy(utils.MinMy(this.rtt, 200), 10)

	if (timeNow-this.timeLastBitrateChange >= bitrateReductionInterval) {
		return true
	}

	if (this.ValidEstimate()) {
		// TODO(terelius/holmer): Investigate consequences of increasing
		// the threshold to 0.95 * LatestEstimate().
		threshold := uint32(0.5 * float64(this.LatestEstimate()))

		return incomingBitrateBps < threshold;
	}

	return false;
}

func (this *AimdRateControl) Update(input *RateControlInput, nowMs int64) {
	if (!this.bitrateIsInitialized) {
		const initializationTimeMs = 5000

		// MS_ASSERT(BitrateWindowMs <= InitializationTimeMs);

		if (this.timeFirstIncomingEstimate < 0) {
			if (input.IncomingBitrate != 0) {
				this.timeFirstIncomingEstimate = nowMs
			}

		} else if (nowMs-this.timeFirstIncomingEstimate > initializationTimeMs && (input.IncomingBitrate != 0)) {
			this.currentBitrateBps = input.IncomingBitrate
			this.bitrateIsInitialized = true
		}
	}

	if (this.updated && this.currentInput.BwState == BW_OVERUSING) {
		// Only update delay factor and incoming bit rate. We always want to react
		// on an over-use.
		this.currentInput.NoiseVar        = input.NoiseVar
		this.currentInput.IncomingBitrate = input.IncomingBitrate
	} else
	{
		this.updated      = true
		this.currentInput = *input
	}
}

func (this *AimdRateControl) GetNearMaxIncreaseRateBps() int{
	responseTime := (this.rtt + 100) * 2
	const MinIncreaseRateBps = 4000.0

	bitsPerFrame     := float64((this.currentBitrateBps) / 30.0)
	packetsPerFrame   := float64(math.Ceil(bitsPerFrame / (8.0 * 1200.0)))
	avgPacketSizeBits := float64(bitsPerFrame / packetsPerFrame)

	return int((math.Max(MinIncreaseRateBps, (avgPacketSizeBits * 1000) / float64(responseTime))))
}

func (this *AimdRateControl) ChangeBitrate(newBitrateBps uint32, incomingBitrateBps uint32, nowMs int64) uint32 {
	if (!this.updated) {
		return this.currentBitrateBps
	}

	if (!this.bitrateIsInitialized && this.currentInput.BwState != BW_OVERUSING) {
		return this.currentBitrateBps
	}

	this.updated = false
	this.ChangeState(&this.currentInput, nowMs)

	incomingBitrateKbps := float64(incomingBitrateBps / 1000.0)
	stdMaxBitRate := math.Sqrt(float64(this.varMaxBitrateKbps * this.avgMaxBitrateKbps))

	switch (this.rateControlState) {
	case RC_HOLD:
	case RC_INCREASE:
		if (this.avgMaxBitrateKbps >= 0 && incomingBitrateKbps > float64(this.avgMaxBitrateKbps) + 3 * stdMaxBitRate) {
			this.ChangeRegion(RC_MAX_UNKNOWN)
			this.avgMaxBitrateKbps = -1.0
		}
		if (this.rateControlRegion == RC_NEAR_MAX) {
			additiveIncreaseBps := this.AdditiveRateIncrease(nowMs, this.timeLastBitrateChange)

			newBitrateBps += additiveIncreaseBps;
		} else
		{
			 multiplicativeIncreaseBps := this.MultiplicativeRateIncrease(nowMs, this.timeLastBitrateChange, newBitrateBps);

			newBitrateBps += multiplicativeIncreaseBps
		}

		this.timeLastBitrateChange = nowMs
	case RC_DECREASE:
		this.bitrateIsInitialized = true
		// Set bit rate to something slightly lower than max to get rid
		// of any self-induced delay.
		newBitrateBps = uint32( utils.LroundMy(float64(this.beta) * float64(incomingBitrateBps) + 0.5))

		if (newBitrateBps > this.currentBitrateBps) {
			// Avoid increasing the rate when over-using.
			if (this.rateControlRegion != RC_MAX_UNKNOWN) {
				newBitrateBps = uint32(utils.LroundMy(float64(this.beta) * float64(this.avgMaxBitrateKbps) * 1000 + 0.5))
			}

			newBitrateBps = uint32(utils.MinMy(int64(newBitrateBps), int64(this.currentBitrateBps)))
		}

		this.ChangeRegion(RC_NEAR_MAX)

		if (incomingBitrateBps < this.currentBitrateBps){
			this.lastDecrease = int(this.currentBitrateBps - newBitrateBps)
		}


		if (incomingBitrateKbps < float64(this.avgMaxBitrateKbps) - 3 * stdMaxBitRate){
			this.avgMaxBitrateKbps = -1.0
		}


		this.UpdateMaxBitRateEstimate(float32(incomingBitrateKbps))
		// Stay on hold until the pipes are cleared.
		this.ChangeStateBase(RC_HOLD)
		this.timeLastBitrateChange = nowMs
	default:
		mylog.Logger.Errorf("invalid this->rateControlState value \n")

	}

	return this.ClampBitrate(newBitrateBps, incomingBitrateBps)

}

func (this *AimdRateControl) ClampBitrate(newBitrateBps,incomingBitrateBps uint32) uint32 {
	 maxBitrateBps := uint32(1.5 * float64(incomingBitrateBps)) + 10000

	if (newBitrateBps > this.currentBitrateBps && newBitrateBps > maxBitrateBps){
		newBitrateBps = uint32(utils.MaxMy(int64(this.currentBitrateBps), int64(maxBitrateBps)))
	}

	newBitrateBps = uint32(utils.MaxMy(int64(newBitrateBps), int64(this.minConfiguredBitrateBps)))

	return newBitrateBps;
}

func (this *AimdRateControl) MultiplicativeRateIncrease(nowMs int64, lastMs int64, currentBitrateBps uint32) uint32 {

	 alpha :=  1.08
	if (lastMs > -1) {
		 timeSinceLastUpdateMs := utils.MinMy(nowMs - lastMs, 1000);

		alpha = math.Pow(alpha, float64(timeSinceLastUpdateMs / 1000.0))
	}
	 multiplicativeIncreaseBps := uint32(math.Max(float64(currentBitrateBps) * (alpha - 1.0), 1000.0))

	return multiplicativeIncreaseBps;
}

func (this *AimdRateControl) UpdateMaxBitRateEstimate(incomingBitrateKbps float32)  {
	  alpha := float32(0.05)

	if (this.avgMaxBitrateKbps == -1.0) {
	this.avgMaxBitrateKbps = incomingBitrateKbps
	} else
	{
	this.avgMaxBitrateKbps = (1.0 - alpha) * this.avgMaxBitrateKbps + alpha * incomingBitrateKbps
	}

	// Estimate the max bit rate variance and normalize the variance
	// with the average max bit rate.
	  norm := float32(math.Max(float64(this.avgMaxBitrateKbps), 1.0))

	this.varMaxBitrateKbps = (1 - alpha) * this.varMaxBitrateKbps + alpha * (this.avgMaxBitrateKbps - incomingBitrateKbps) * (this.avgMaxBitrateKbps - incomingBitrateKbps) / norm

	// 0.4 ~= 14 kbit/s at 500 kbit/s
	if (this.varMaxBitrateKbps < 0.4){
		this.varMaxBitrateKbps = 0.4
	}

	// 2.5f ~= 35 kbit/s at 500 kbit/s
	if (this.varMaxBitrateKbps > 2.5){
		this.varMaxBitrateKbps = 2.5
	}

}

func (this *AimdRateControl) ChangeState(input *RateControlInput, nowMs int64)  {
	switch (this.currentInput.BwState){
	case BW_NORMAL:
		if (this.rateControlState == RC_HOLD) {
			this.timeLastBitrateChange = nowMs
			this.ChangeStateBase(RC_INCREASE)
		}
	case BW_OVERUSING:
		if (this.rateControlState != RC_DECREASE) {
			this.ChangeStateBase(RC_DECREASE)
		}
	case BW_UNDERUSING:
		this.ChangeStateBase(RC_HOLD)
	default:
		mylog.Logger.Errorf("invalid RateControlInput::bwState value \n")
	}
}
