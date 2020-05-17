package YCP

import "code.int-2.me/yuyyi51/YCP/packet"

type CongestionAlgorithm interface {
	OnPacketsSend([]PacketInfo)
	OnPacketsAck([]PacketInfo)
	OnPacketsLost([]PacketInfo)
	GetCongestionWindow() uint64
}

type PacketInfo struct {
	seq  uint64
	size int
}

const (
	SlowStart       = 0
	CongestionAvoid = 1
)

type RenoAlgorithm struct {
	cwnd               uint64
	status             int
	slowStartThreshold uint64
}

func NewRenoAlgorithm() *RenoAlgorithm {
	return &RenoAlgorithm{
		cwnd:               10 * packet.Ipv4PayloadSize,
		status:             SlowStart,
		slowStartThreshold: 32 * packet.Ipv4PayloadSize,
	}
}

func (r *RenoAlgorithm) OnPacketsSend(pkts []PacketInfo) {
	if r.status == CongestionAvoid {
		r.cwnd += uint64(len(pkts))
	}
}

func (r *RenoAlgorithm) OnPacketsAck(pkts []PacketInfo) {
	if r.cwnd < r.slowStartThreshold {
		r.cwnd += uint64(len(pkts))
	} else {
		r.status = CongestionAvoid
	}
}

func (r *RenoAlgorithm) OnPacketsLost(pkts []PacketInfo) {
	r.slowStartThreshold = r.cwnd >> 1
	r.cwnd = 10 * packet.Ipv4PayloadSize
	r.status = SlowStart
}

func (r *RenoAlgorithm) GetCongestionWindow() uint64 {
	return r.cwnd
}
