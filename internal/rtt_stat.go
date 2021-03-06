package internal

import (
	"github.com/yuyyi51/YCP/packet"
	"time"
)

type RttStat struct {
	smoothRtt time.Duration
	rttVar    time.Duration
	rto       time.Duration
}

func (stat *RttStat) Init(rtt time.Duration) {
	stat.smoothRtt = rtt
	stat.rttVar = rtt
	stat.rto = 3 * rtt
}

func (stat *RttStat) Update(rtt time.Duration) {
	if stat.smoothRtt == 0 {
		stat.Init(rtt)
		return
	}
	e := stat.smoothRtt - rtt
	if e < 0 {
		e = -e
	}
	stat.rttVar = 3*stat.rttVar/4 + e/4
	stat.smoothRtt = 7*stat.smoothRtt/8 + rtt/8
	stat.rto = stat.smoothRtt + 4*stat.rttVar
}

func (stat *RttStat) SmoothRtt() time.Duration {
	return stat.smoothRtt
}

func (stat *RttStat) Rto() time.Duration {
	if stat.rto == 0 {
		return packet.SessionInitRto
	}
	return stat.rto
}
