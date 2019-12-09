package compoundrtcp
import (

	"github.com/pion/rtcp"
)
type CompoundRtcp struct {
	Packet []rtcp.Packet
	DateLen int
}

func NewCompoundRtcp() *CompoundRtcp{
	return &CompoundRtcp{Packet:make(rtcp.CompoundPacket,0,6),DateLen:0}
}


func (this *CompoundRtcp)AddReceiverReport(report *rtcp.ReceiverReport){
	this.Packet = append(this.Packet,report)
	this.DateLen += report.Getlen()
}

func (this *CompoundRtcp)AddSenderReport(report *rtcp.SenderReport){
	this.Packet = append(this.Packet,report)
	this.DateLen += report.Getlen()
}

func (this *CompoundRtcp)AddSdes(report *rtcp.SourceDescription){
	this.Packet = append(this.Packet,report)
	this.DateLen += report.Getlen()
}

