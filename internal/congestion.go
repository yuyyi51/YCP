package internal

import (
	"code.int-2.me/yuyyi51/YCP/packet"
	"code.int-2.me/yuyyi51/ylog"
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
	Round           uint64
}

func PacketToInfo(pkt *PacketHistoryItem) PacketInfo {
	return PacketInfo{
		Seq:             pkt.seq,
		Size:            pkt.packet.Size(),
		Retransmittable: pkt.retransmittable,
		Round:           pkt.round,
	}
}

func PacketsToInfo(pkts []*PacketHistoryItem) []PacketInfo {
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
	maxSent            int64
	logger             ylog.ILogger
	largestRound       uint64
	lastDecreaseSeq    int64
}

func NewRenoAlgorithm(logger ylog.ILogger) *RenoAlgorithm {
	return &RenoAlgorithm{
		cwnd:               100 * packet.Ipv4PayloadSize,
		status:             SlowStart,
		slowStartThreshold: 200 * packet.Ipv4PayloadSize,
		logger:             logger,
		lastDecreaseSeq:    -1,
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
		if pkt.Round > r.largestRound {
			newRound = true
			r.largestRound = pkt.Round
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
	r.logger.Debug("Reno OnPacketsAck cwnd: %d, round: %d", r.cwnd, r.largestRound)
}

func (r *RenoAlgorithm) OnPacketsLost(pkts []PacketInfo) {
	if len(pkts) != 0 {
		lostSeqs := make([]uint64, 0, len(pkts))
		maxLostSeq := int64(0)
		for _, pkt := range pkts {
			lostSeqs = append(lostSeqs, pkt.Seq)
			if int64(pkt.Seq) > maxLostSeq {
				maxLostSeq = int64(pkt.Seq)
			}
		}
		r.logger.Debug("Reno OnPacketsLost num: %d, %v", len(lostSeqs), lostSeqs)
		if maxLostSeq > r.lastDecreaseSeq {
			r.lastDecreaseSeq = -1
			r.slowStartThreshold = r.cwnd >> 1
			r.cwnd = r.slowStartThreshold
			r.status = SlowStart
		}
		r.logger.Debug("Reno OnPacketsLost cwnd: %d", r.cwnd)
	}
}

func (r *RenoAlgorithm) GetCongestionWindow() int64 {
	return r.cwnd
}
