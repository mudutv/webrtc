package rembServer

import (
	"github.com/pion/webrtc/deque"
	"math"
	"github.com/pion/webrtc/mylog"
)

const MinFramePeriodHistoryLength  = 60
const DeltaCounterMax  = 1000
type OverUseDetectorOptions struct {
	 InitialSlope float64
	 InitialOffset float64
	 InitialE[2][2] float64
	 InitialProcessNoise[2] float64
	 InitialAvgNoise float64
	 InitialVarNoise float64
}

func NewOverUseDetectorOptions() OverUseDetectorOptions{
	node := OverUseDetectorOptions{}

	node.InitialSlope = 8.0 / 512.0
	node.InitialOffset = 0
	node.InitialAvgNoise = 0.0
	node.InitialVarNoise = 50

	node.InitialE[0][0] = 100
	node.InitialE[1][1] = 1e-1
	node.InitialE[0][1] = 0
	node.InitialE[1][0] = 0
	node.InitialProcessNoise[0] = 1e-13
	node.InitialProcessNoise[1] = 1e-3

	return node

}

type OveruseEstimator struct {
	 options OverUseDetectorOptions
	 numOfDeltas uint16
	 slope float64
	 offset float64 
	 prevOffset float64
	 e[2][2] float64
	 processNoise[2] float64
	 avgNoise float64
	 varNoise float64
	 tsDeltaHist deque.Deque
}

func NewOveruseEstimator(options OverUseDetectorOptions) OveruseEstimator{
	node := OveruseEstimator{}
	node.options = options
	node.numOfDeltas = 0
	node.slope = node.options.InitialSlope
	node.offset = node.options.InitialOffset
	node.prevOffset = node.options.InitialOffset
	node.avgNoise = node.options.InitialAvgNoise
	node.varNoise = node.options.InitialAvgNoise

	node.e = node.options.InitialE
	node.processNoise = node.options.InitialProcessNoise

	return node
}

func (this *OveruseEstimator)GetVarNoise() float64{
	return this.varNoise
}

func (this *OveruseEstimator)GetOffset() float64{
	return this.offset
}

func (this *OveruseEstimator)GetNumOfDeltas() uint16{
	return this.numOfDeltas
}

func (this *OveruseEstimator)Update(tDelta int64, tsDelta float64, sizeDelta int, currentHypothesis int, nowMs int64){
	 minFramePeriod := float64(this.UpdateMinFramePeriod(tsDelta))
	 tTsDelta       := float64(tDelta) - tsDelta
	 fsDelta             := float64(sizeDelta)

	this.numOfDeltas++

	if (this.numOfDeltas > DeltaCounterMax){
		this.numOfDeltas = DeltaCounterMax
	}


	// Update the Kalman filter.
	this.e[0][0] += this.processNoise[0]
	this.e[1][1] += this.processNoise[1]

	if (
		(currentHypothesis == BW_OVERUSING && this.offset < this.prevOffset) ||
			(currentHypothesis == BW_UNDERUSING && this.offset > this.prevOffset)) {
		this.e[1][1] += 10 * this.processNoise[1]
	}

     h       := [2]float64{ fsDelta, 1.0 }
	 eh       := [2]float64{ this.e[0][0] * h[0] + this.e[0][1] * h[1],
	this.e[1][0] * h[0] + this.e[1][1] * h[1] }
	 residual    := float64(tTsDelta - this.slope * h[0] - this.offset)
	inStableState := (currentHypothesis == BW_NORMAL)
	 maxResidual := 3.0 * math.Sqrt(this.varNoise)

	// We try to filter out very late frames. For instance periodic key
	// frames doesn't fit the Gaussian model well.
	if (math.Abs(residual) < maxResidual) {
		this.UpdateNoiseEstimate(residual, minFramePeriod, inStableState);
	} else
	{
		a := float64(0)
		if (residual < 0){
			a =  -maxResidual
		}else {
			a = maxResidual
		}
		this.UpdateNoiseEstimate(a , minFramePeriod, inStableState)
	}

	denom     := float64(this.varNoise + h[0] * eh[0] + h[1] * eh[1])
	k      := []float64{ eh[0] / denom, eh[1] / denom }
	 iKh := [2][2]float64{ { 1.0 - k[0] * h[0], -k[0] * h[1] },
	{ -k[1] * h[0], 1.0 - k[1] * h[1] } }
	 e00       := this.e[0][0]
	 e01       := this.e[0][1]

	// Update state.
	this.e[0][0] = e00 * iKh[0][0] + this.e[1][0] * iKh[0][1]
	this.e[0][1] = e01 * iKh[0][0] + this.e[1][1] * iKh[0][1]
	this.e[1][0] = e00 * iKh[1][0] + this.e[1][0] * iKh[1][1]
	this.e[1][1] = e01 * iKh[1][0] + this.e[1][1] * iKh[1][1]

	// The covariance matrix must be positive semi-definite.
	 positiveSemiDefinite :=
		this.e[0][0] + this.e[1][1] >= 0 &&
			this.e[0][0] * this.e[1][1] - this.e[0][1] * this.e[1][0] >= 0 && this.e[0][0] >= 0


	if (!positiveSemiDefinite) {
		mylog.Logger.Error("the over-use estimator's covariance matrix is no longer semi-definite")
	}

	this.slope      = this.slope + k[0] * residual
	this.prevOffset = this.offset
	this.offset     = this.offset + k[1] * residual
}

func (this *OveruseEstimator)UpdateMinFramePeriod(tsDelta float64) float64{
	 minFramePeriod := float64(tsDelta)

	if (this.tsDeltaHist.Len() >= MinFramePeriodHistoryLength){
		this.tsDeltaHist.PopFront()
	}


	for i := 0; i <   this.tsDeltaHist.Len();i++{
		date := this.tsDeltaHist.At(i)
		oldTsDelta := date.(float64)
		minFramePeriod = math.Min(oldTsDelta, minFramePeriod)
	}


	this.tsDeltaHist.PushBack(tsDelta)

	return minFramePeriod
}

func (this *OveruseEstimator)UpdateNoiseEstimate(residual float64, tsDelta float64, stableState bool) {
	if (!stableState){
		return
	}


	// Faster filter during startup to faster adapt to the jitter level
	// of the network. |alpha| is tuned for 30 frames per second, but is scaled
	// according to |tsDelta|.
	 alpha := float64(0.01)

	if (this.numOfDeltas > 10 * 30){
		alpha = 0.002
	}


	// Only update the noise estimate if we're not over-using. |beta| is a
	// function of alpha and the time delta since the previous update.
	 beta := math.Pow(1.0 - alpha, tsDelta * 30.0 / 1000.0)

	this.avgNoise = beta * this.avgNoise + (1 - beta) * residual
	this.varNoise = beta * this.varNoise +
		(1 - beta) * (this.avgNoise - residual) * (this.avgNoise - residual)
	if (this.varNoise < 1){
		this.varNoise = 1
	}

}
