package internal

import (
	"code.int-2.me/yuyyi51/YCP/packet"
	"code.int-2.me/yuyyi51/ylog"
	"sync"
	"time"
)

type PacketHistory struct {
	itemMap      map[uint64]*PacketHistoryItem
	mapMux       *sync.RWMutex
	bytesSent    int64
	inflight     int64
	bytesAck     int64
	bytesLost    int64
	logger       ylog.ILogger
	currentRound uint64
	lastRoundSeq uint64
	maxSeq       uint64
}

type PacketHistoryItem struct {
	seq             uint64
	retransmittable bool
	packet          packet.Packet
	acked           bool
	sentTime        time.Time
	rtoTime         time.Time
	queuedRto       bool
	fastRetrans     int
	isFin           bool
	round           uint64
}

type AckInfo struct {
	Seq             uint64
	Rtt             time.Duration
	Retransmittable bool
}

func NewPacketHistory(logger ylog.ILogger) *PacketHistory {
	return &PacketHistory{
		itemMap: map[uint64]*PacketHistoryItem{},
		mapMux:  new(sync.RWMutex),
		logger:  logger,
	}
}

func (history *PacketHistory) SendPacket(pkt packet.Packet, rto time.Duration) {
	history.mapMux.Lock()
	defer history.mapMux.Unlock()
	item := &PacketHistoryItem{
		seq:             pkt.Seq,
		retransmittable: pkt.IsRetransmittable(),
		packet:          pkt,
		sentTime:        time.Now(),
		rtoTime:         time.Now().Add(rto),
		round:           history.currentRound,
	}
	history.maxSeq = pkt.Seq
	history.itemMap[pkt.Seq] = item
	if pkt.IsRetransmittable() {
		history.bytesSent += int64(pkt.Size())
	}
}

func (history *PacketHistory) SendRetransmitPacket(pkt packet.Packet, rto time.Duration, retransmitFor uint64) {
	history.mapMux.Lock()
	defer history.mapMux.Unlock()
	item := &PacketHistoryItem{
		seq:             pkt.Seq,
		retransmittable: pkt.IsRetransmittable(),
		packet:          pkt,
		sentTime:        time.Now(),
		rtoTime:         time.Now().Add(rto),
		round:           history.currentRound,
	}
	history.maxSeq = pkt.Seq
	history.itemMap[pkt.Seq] = item
	if pkt.IsRetransmittable() {
		history.bytesSent += int64(pkt.Size())
	}
	delete(history.itemMap, retransmitFor)
}

func (history *PacketHistory) AckPackets(ranges []packet.AckRange) []PacketInfo {
	history.mapMux.Lock()
	defer history.mapMux.Unlock()
	ackedPackets := make([]PacketInfo, 0)
	for _, ran := range ranges {
		for i := ran.Left; i <= ran.Right; i++ {
			_, ok := history.itemMap[i]
			if !ok {
				continue
			}
			if i > history.lastRoundSeq {
				// new round
				history.logger.Debug("new round [%d], start with %d, lastRoundSeq: %d, newly acked: %d", history.currentRound+1, history.maxSeq+1, history.currentRound, i)
				history.lastRoundSeq = history.maxSeq + 1
				history.currentRound++
			}
			if history.itemMap[i].packet.IsRetransmittable() && !history.itemMap[i].queuedRto {
				history.bytesAck += int64(history.itemMap[i].packet.Size())
			}
			newlyAcked := PacketInfo{
				Seq:             i,
				Rtt:             time.Since(history.itemMap[i].sentTime),
				Retransmittable: history.itemMap[i].retransmittable,
				Size:            history.itemMap[i].packet.Size(),
				Round:           history.itemMap[i].round,
			}
			ackedPackets = append(ackedPackets, newlyAcked)
			delete(history.itemMap, i)
			for j := range history.itemMap {
				if j < i {
					history.itemMap[j].fastRetrans++
				}
			}
		}
	}
	return ackedPackets
}

func (history *PacketHistory) Inflight() int64 {
	return history.bytesSent - history.bytesAck - history.bytesLost
}

func (history *PacketHistory) IsInflight(seq uint64) bool {
	_, ok := history.itemMap[seq]
	return ok
}

func (history *PacketHistory) FindTimeoutPacket() []*PacketHistoryItem {
	history.mapMux.Lock()
	defer history.mapMux.Unlock()
	needRetransmit := make([]*PacketHistoryItem, 0)
	for i, pkt := range history.itemMap {
		if !pkt.queuedRto && pkt.rtoTime.Before(time.Now()) {
			needRetransmit = append(needRetransmit, pkt)
			pkt.queuedRto = true
			if pkt.retransmittable {
				history.bytesLost += int64(pkt.packet.Size())
			} else {
				// not retransmittable packet, remove from history
				delete(history.itemMap, i)
			}
		}
	}
	return needRetransmit
}

func (history *PacketHistory) FindFastRetransmitPacket() []*PacketHistoryItem {
	history.mapMux.Lock()
	defer history.mapMux.Unlock()
	needRetransmit := make([]*PacketHistoryItem, 0)
	for i, pkt := range history.itemMap {
		if !pkt.queuedRto && pkt.fastRetrans >= packet.FastTransmitCount {
			needRetransmit = append(needRetransmit, pkt)
			pkt.queuedRto = true
			if pkt.retransmittable {
				history.bytesLost += int64(pkt.packet.Size())
			} else {
				// not retransmittable packet, remove from history
				delete(history.itemMap, i)
			}
		}
	}
	return needRetransmit
}

func ExtractPktFromItems(items []*PacketHistoryItem) []packet.Packet {
	res := make([]packet.Packet, 0, len(items))
	for _, item := range items {
		res = append(res, item.packet)
	}
	return res
}

func LogPacketSeq(pkts []*PacketHistoryItem) []uint64 {
	seqs := make([]uint64, 0, len(pkts))
	for i := range pkts {
		seqs = append(seqs, pkts[i].seq)
	}
	return seqs
}
