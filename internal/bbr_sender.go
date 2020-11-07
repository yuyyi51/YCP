package internal

import (
	"github.com/yuyyi51/ylog"
	"time"
)

type BbrSender struct {
	pacingSender *PacingSender
	logger       ylog.ILogger
	band         float32
	minRtt       time.Duration
	history      *BbrHistory
	bandSampler  bandwidthSampler
	rttSampler   rttSampler
	lastestRound uint64
}

func NewBbrSender(logger ylog.ILogger) *BbrSender {
	sender := &BbrSender{
		logger:  logger,
		history: newBbrHistory(),
	}
	sender.pacingSender = NewPacingSender(sender.GetBandwidth, logger)
	// init rtt 2Mbps
	sender.bandSampler.reset(2000*1024/8, 0)
	sender.band = sender.bandSampler.max()
	return sender
}

func (b *BbrSender) OnPacketsSend(infos []PacketInfo) {
	round := uint64(0)
	for _, info := range infos {
		b.history.onSend(info)
		b.history.totalSendBytes += uint64(info.Size)
		if round < info.Round {
			round = info.Round
		}
	}
	if round > b.lastestRound {
		b.lastestRound = round
		b.band = b.bandSampler.max() * 2.25
	}
	b.logger.Debug("BbrSender OnPacketsSend, round: %d, lastestRound: %d, ban: %s, maxBan: %s",
		round,
		b.lastestRound,
		IntoBps(b.band),
		IntoBps(b.bandSampler.max()))
}

func (b *BbrSender) OnPacketsAck(infos []PacketInfo) {
	for _, info := range infos {
		b.history.totalAckBytes += uint64(info.Size)
		newBand, rtt := b.history.onAck(info.Seq)
		if rtt == 0 {
			continue
		}
		b.bandSampler.input(newBand, b.lastestRound)
		b.rttSampler.input(rtt, b.lastestRound)
		b.logger.Debug("BbrSender OnPacketsAck, newBand: %s, rtt: %s, maxBand: %s", IntoBps(newBand), rtt, IntoBps(b.bandSampler.max()))
	}
	b.minRtt = b.rttSampler.min()
}

func (b *BbrSender) OnPacketsLost(infos []PacketInfo) {
	// todo: calc lost rate
	for _, info := range infos {
		b.history.onLost(info.Seq)
	}
}

func (b *BbrSender) GetCongestionWindow() int64 {
	return 1 << 62
}

func (b *BbrSender) UpdateRtt(duration time.Duration) {

}

func (b *BbrSender) UpdatePacingThreshold() {
	b.pacingSender.UpdatePacingBytes()
}

func (b *BbrSender) PacingSend(size int64) bool {
	return b.pacingSender.PacingSend(size)
}

func (b *BbrSender) PacingBytes() int64 {
	return b.pacingSender.pacingBytes
}

func (b *BbrSender) GetBandwidth() float32 {
	return b.band
}

type bandwidthSampler struct {
	round uint64
	band  [3]float32
}

func (bs *bandwidthSampler) input(sample float32, round uint64) {
	if round > bs.round {
		bs.band[round%3] = sample
		bs.round = round
		return
	}
	if bs.band[round%3] < sample {
		bs.band[round%3] = sample
	}
}

func (bs *bandwidthSampler) reset(sample float32, round uint64) {
	for i := range bs.band {
		bs.band[i] = sample
	}
	bs.round = round
}

func (bs *bandwidthSampler) max() float32 {
	var maxBand float32
	for i := range bs.band {
		if maxBand < bs.band[i] {
			maxBand = bs.band[i]
		}
	}
	return maxBand
}

type rttSampler struct {
	round uint64
	rtt   [3]time.Duration
}

func (rs *rttSampler) input(sample time.Duration, round uint64) {
	if round > rs.round {
		rs.rtt[round%3] = sample
		rs.round = round
		return
	}
	if rs.rtt[round%3] > sample {
		rs.rtt[round%3] = sample
	}
}

func (rs *rttSampler) reset(sample time.Duration, round uint64) {
	for i := range rs.rtt {
		rs.rtt[i] = sample
	}
	rs.round = round
}

func (rs *rttSampler) min() time.Duration {
	var minRtt time.Duration
	for i := range rs.rtt {
		if minRtt > rs.rtt[i] {
			minRtt = rs.rtt[i]
		}
	}
	return minRtt
}
