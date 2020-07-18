package internal

import (
	"code.int-2.me/yuyyi51/YCP/packet"
	"time"
)

type CongestionAlgorithm interface {
	OnPacketsSend([]PacketInfo)
	OnPacketsAck([]PacketInfo)
	OnPacketsLost([]PacketInfo)
	GetCongestionWindow() int64
}

type PacketInfo struct {
	Seq             uint64
	Size            int
	Rtt             time.Duration
	Retransmittable bool
}

func PacketToInfo(pkt packet.Packet) PacketInfo {
	return PacketInfo{
		Seq:             pkt.Seq,
		Size:            pkt.Size(),
		Retransmittable: pkt.IsRetransmittable(),
	}
}

func PacketsToInfo(pkts []packet.Packet) []PacketInfo {
	infos := make([]PacketInfo, 0, len(pkts))
	for _, pkt := range pkts {
		infos = append(infos, PacketToInfo(pkt))
	}
	return infos
}

const (
	SlowStart       = 0
	CongestionAvoid = 1
)

type RenoAlgorithm struct {
	cwnd               int64
	status             int
	slowStartThreshold int64
	round              int64
	lastRoundPkt       int64
	maxSent            int64
}

func NewRenoAlgorithm() *RenoAlgorithm {
	return &RenoAlgorithm{
		cwnd:               32 * packet.Ipv4PayloadSize,
		status:             SlowStart,
		slowStartThreshold: 64 * packet.Ipv4PayloadSize,
	}
}

func (r *RenoAlgorithm) OnPacketsSend(pkts []PacketInfo) {
	for _, pkt := range pkts {
		if int64(pkt.Seq) > r.maxSent {
			r.maxSent = int64(pkt.Seq)
		}
	}
}

func (r *RenoAlgorithm) OnPacketsAck(pkts []PacketInfo) {
	newRound := false
	for _, pkt := range pkts {
		if int64(pkt.Seq) > r.lastRoundPkt {
			// new round
			newRound = true
			r.lastRoundPkt = r.maxSent
		}
	}

	if r.cwnd < r.slowStartThreshold {
		for _, pkt := range pkts {
			r.cwnd += int64(pkt.Size)
		}
	} else {
		r.status = CongestionAvoid
		if newRound {
			r.cwnd += packet.Ipv4PayloadSize
		}
	}
}

func (r *RenoAlgorithm) OnPacketsLost(pkts []PacketInfo) {
	if len(pkts) != 0 {
		r.slowStartThreshold = r.cwnd >> 1
		r.cwnd = 32 * packet.Ipv4PayloadSize
		r.status = SlowStart
	}
}

func (r *RenoAlgorithm) GetCongestionWindow() int64 {
	return r.cwnd
}
