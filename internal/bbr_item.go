package internal

import "time"

type BbrItem struct {
	seq       uint64
	ackBytes  uint64
	sendBytes uint64
	sendTime  time.Time
	round     uint64
}
