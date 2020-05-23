package internal

import "code.int-2.me/yuyyi51/YCP/packet"

type PacketHistory struct {
	itemMap map[uint64]PacketHistoryItem
}

type PacketHistoryItem struct {
	seq              uint64
	retransmittable  bool
	retransmittedAs  *PacketHistoryItem
	retransmittedfor *PacketHistoryItem
	packet           packet.Packet
}

func (history *PacketHistory) SendPacket(pkt packet.Packet) {
	item := PacketHistoryItem{
		seq:             pkt.Seq,
		retransmittable: pkt.IsRetransmittable(),
		packet:          pkt,
	}
	history.itemMap[pkt.Seq] = item
}

func (history *PacketHistory) SendRetransmitPacket(pkt packet.Packet, retransmitFor uint64) {

}

func (history *PacketHistory) AckPacket(seq uint64) {

}
