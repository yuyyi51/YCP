package utils

import "time"

type CostTimer struct {
	start time.Time
}

func NewCostTimer() CostTimer {
	return CostTimer{
		start: time.Now(),
	}
}

func (ct *CostTimer) Cost() time.Duration {
	return time.Since(ct.start)
}

func (ct *CostTimer) Reset() {
	ct.start = time.Now()
}
