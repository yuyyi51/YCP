package internal

import "time"

type BbrHistory struct {
	itemMap        map[uint64]*BbrItem
	totalAckBytes  uint64
	totalSendBytes uint64
	minRtt         time.Duration
}

func (h *BbrHistory) packetInfoToItem(info PacketInfo) *BbrItem {
	return &BbrItem{
		seq:       info.Seq,
		round:     info.Round,
		ackBytes:  h.totalAckBytes,
		sendBytes: h.totalSendBytes,
		sendTime:  time.Now(),
	}
}

func newBbrHistory() *BbrHistory {
	return &BbrHistory{
		itemMap: map[uint64]*BbrItem{},
	}
}

func (h *BbrHistory) onSend(info PacketInfo) {
	item := h.packetInfoToItem(info)
	h.itemMap[item.seq] = item
}

func (h *BbrHistory) onAck(seq uint64) (float32, time.Duration) {
	item, ok := h.itemMap[seq]
	if !ok {
		return 0, 0
	}
	rtt := time.Since(item.sendTime)
	sendDelta := h.totalSendBytes - item.sendBytes
	sendRate := float32(sendDelta) * float32(time.Second) / float32(rtt)
	ackDelta := h.totalAckBytes - item.ackBytes
	ackRate := float32(ackDelta) * float32(time.Second) / float32(rtt)
	var finalRate float32
	if ackRate < sendRate {
		finalRate = ackRate
	} else {
		finalRate = sendRate
	}
	if h.minRtt == 0 || h.minRtt > rtt {
		h.minRtt = rtt
	}
	return finalRate, rtt
}

func (h *BbrHistory) onLost(seq uint64) {
	delete(h.itemMap, seq)
}
