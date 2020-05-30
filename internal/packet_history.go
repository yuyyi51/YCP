package internal

import (
	"code.int-2.me/yuyyi51/YCP/packet"
	"fmt"
	"time"
)

type PacketHistory struct {
	itemMap    map[uint64]*PacketHistoryItem
	maxUna     uint64
	bytesSent  uint64
	inflight   uint64
	bytesAcked uint64
}

type PacketHistoryItem struct {
	seq              uint64
	retransmittable  bool
	retransmittedAs  *PacketHistoryItem
	retransmittedfor *PacketHistoryItem
	packet           packet.Packet
	acked            bool
	sentTime         time.Time
}

func NewPacketHistory() *PacketHistory {
	return &PacketHistory{
		itemMap: map[uint64]*PacketHistoryItem{},
	}
}

func (history *PacketHistory) SendPacket(pkt packet.Packet) {
	item := &PacketHistoryItem{
		seq:             pkt.Seq,
		retransmittable: pkt.IsRetransmittable(),
		packet:          pkt,
		sentTime:        time.Now(),
	}
	history.itemMap[pkt.Seq] = item
	history.bytesSent += uint64(pkt.Size())
}

func (history *PacketHistory) SendRetransmitPacket(pkt packet.Packet, retransmitFor uint64) {

}

func (history *PacketHistory) AckPackets(ranges []packet.AckRange) {
	for _, ran := range ranges {
		for i := ran.Left; i <= ran.Right; i++ {
			if i < history.maxUna || history.itemMap[i].acked {
				continue
			}
			history.itemMap[i].acked = true
			history.bytesAcked += uint64(history.itemMap[i].packet.Size())
			fmt.Printf("Acked new packet [%d], rtt: %s\n", i, time.Since(history.itemMap[i].sentTime))
		}
	}
	history.updateUna()
}

func (history *PacketHistory) Inflight() uint64 {
	return history.bytesSent - history.bytesAcked
}

func (history *PacketHistory) updateUna() {
	for {
		pkt, ok := history.itemMap[history.maxUna]
		if !ok || !pkt.acked {
			break
		}
		history.maxUna++
	}
}
