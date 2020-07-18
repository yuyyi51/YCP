package internal

import (
	"code.int-2.me/yuyyi51/YCP/packet"
	"code.int-2.me/yuyyi51/YCP/utils"
	"sync"
	"time"
)

type PacketHistory struct {
	itemMap   map[uint64]*PacketHistoryItem
	mapMux    *sync.RWMutex
	maxUna    uint64
	bytesSent int64
	inflight  int64
	bytesAck  int64
	bytesLost int64
	logger    *utils.Logger
}

type PacketHistoryItem struct {
	seq             uint64
	retransmittable bool
	packet          packet.Packet
	acked           bool
	sentTime        time.Time
	rtoTime         time.Time
	queuedRto       bool
}

type AckInfo struct {
	Seq             uint64
	Rtt             time.Duration
	Retransmittable bool
}

func NewPacketHistory(logger *utils.Logger) *PacketHistory {
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
	}
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
	}
	history.itemMap[pkt.Seq] = item
	if pkt.IsRetransmittable() {
		history.bytesSent += int64(pkt.Size())
	}
	delete(history.itemMap, retransmitFor)
}

func (history *PacketHistory) AckPackets(ranges []packet.AckRange) []AckInfo {
	history.mapMux.Lock()
	defer history.mapMux.Unlock()
	ackedPackets := make([]AckInfo, 0)
	for _, ran := range ranges {
		for i := ran.Left; i <= ran.Right; i++ {
			_, ok := history.itemMap[i]
			if !ok {
				continue
			}
			if history.itemMap[i].packet.IsRetransmittable() && !history.itemMap[i].queuedRto {
				history.bytesAck += int64(history.itemMap[i].packet.Size())
			}
			newlyAcked := AckInfo{
				Seq:             i,
				Rtt:             time.Since(history.itemMap[i].sentTime),
				Retransmittable: history.itemMap[i].retransmittable,
			}
			ackedPackets = append(ackedPackets, newlyAcked)
			delete(history.itemMap, i)
		}
	}
	return ackedPackets
}

func (history *PacketHistory) Inflight() int64 {
	return history.bytesSent - history.bytesAck - history.bytesLost
}

func (history *PacketHistory) IsInflight(seq uint64) bool {
	_, ok := history.itemMap[seq]
	history.logger.Debug("IsInflight called, itemMap length %d, seq %d", len(history.itemMap), seq)
	return ok
}

func (history *PacketHistory) FindTimeoutPacket() []packet.Packet {
	history.mapMux.Lock()
	defer history.mapMux.Unlock()
	needRetransmit := make([]packet.Packet, 0)
	for i, pkt := range history.itemMap {
		if !pkt.queuedRto && pkt.rtoTime.Before(time.Now()) {
			needRetransmit = append(needRetransmit, pkt.packet)
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
