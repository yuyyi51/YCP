package internal

import (
	"code.int-2.me/yuyyi51/YCP/packet"
	"fmt"
	"sync"
	"time"
)

type PacketHistory struct {
	itemMap    map[uint64]*PacketHistoryItem
	mapMux     *sync.RWMutex
	maxUna     uint64
	bytesSent  uint64
	inflight   uint64
	bytesAcked uint64
}

type PacketHistoryItem struct {
	seq             uint64
	retransmittable bool
	packet          packet.Packet
	acked           bool
	sentTime        time.Time
}

func NewPacketHistory() *PacketHistory {
	return &PacketHistory{
		itemMap: map[uint64]*PacketHistoryItem{},
		mapMux:  new(sync.RWMutex),
	}
}

func (history *PacketHistory) SendPacket(pkt packet.Packet) {
	history.mapMux.Lock()
	defer history.mapMux.Unlock()
	item := &PacketHistoryItem{
		seq:             pkt.Seq,
		retransmittable: pkt.IsRetransmittable(),
		packet:          pkt,
		sentTime:        time.Now(),
	}
	history.itemMap[pkt.Seq] = item
	history.bytesSent += uint64(pkt.Size())
}

func (history *PacketHistory) SendRetransmitPacket(pkt packet.Packet) {
	history.mapMux.Lock()
	defer history.mapMux.Unlock()
	item := &PacketHistoryItem{
		seq:             pkt.Seq,
		retransmittable: pkt.IsRetransmittable(),
		packet:          pkt,
		sentTime:        time.Now(),
	}
	history.itemMap[pkt.Seq] = item
}

func (history *PacketHistory) AckPackets(ranges []packet.AckRange) {
	history.mapMux.Lock()
	defer history.mapMux.Unlock()
	for _, ran := range ranges {
		for i := ran.Left; i <= ran.Right; i++ {
			_, ok := history.itemMap[i]
			if !ok {
				continue
			}
			history.bytesAcked += uint64(history.itemMap[i].packet.Size())
			if history.itemMap[i].retransmittable {
				fmt.Printf("Acked new packet [%d], rtt: %s\n", i, time.Since(history.itemMap[i].sentTime))
			}
			delete(history.itemMap, i)
		}
	}
}

func (history *PacketHistory) Inflight() uint64 {
	return history.bytesSent - history.bytesAcked
}

func (history *PacketHistory) FindTimeoutPacket() []packet.Packet {
	history.mapMux.Lock()
	defer history.mapMux.Unlock()
	needRetransmit := make([]packet.Packet, 0)
	for i, pkt := range history.itemMap {
		if time.Since(pkt.sentTime) > packet.SessionInitRto {
			needRetransmit = append(needRetransmit, pkt.packet)
			delete(history.itemMap, i)
		}
	}
	return needRetransmit
}
