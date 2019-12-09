package rembServer

const (
	BW_NORMAL = iota
	BW_UNDERUSING
	BW_OVERUSING
)
type RateControlInput struct {
	BwState int //BW_NORMAL
	IncomingBitrate uint32
	NoiseVar float64
}

func NewRateControlInput(BwState int,  IncomingBitrate uint32,NoiseVar float64 ) RateControlInput {
	node := RateControlInput{}
	node.BwState = BwState
	node.IncomingBitrate =IncomingBitrate
	node.NoiseVar =NoiseVar
	return node
}