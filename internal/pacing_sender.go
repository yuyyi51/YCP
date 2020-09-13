package internal

import (
	"code.int-2.me/yuyyi51/ylog"
	"fmt"
	"time"
)

type PacingSender struct {
	lastCalledTime time.Time
	// unit must be byte/s
	pacingRateFunc func() float32
	initInterval   time.Duration
	pacingBytes    int64
	logger         ylog.ILogger
}

func NewPacingSender(rateFunc func() float32, logger ylog.ILogger) *PacingSender {
	return &PacingSender{
		pacingRateFunc: rateFunc,
		initInterval:   10 * time.Millisecond,
		logger:         logger,
	}
}

func (s *PacingSender) UpdatePacingBytes() int64 {
	var interval time.Duration
	if s.lastCalledTime.IsZero() {
		interval = s.initInterval
	} else {
		interval = time.Since(s.lastCalledTime)
	}
	s.lastCalledTime = time.Now()
	pacingRate := s.pacingRateFunc()
	pacingBytes := int64(float32(interval) * pacingRate / float32(time.Second))
	s.pacingBytes += pacingBytes
	s.logger.Debug("PacingSender updated, interval: %s, pacingRate: %s, pacingBytes: %d", interval, IntoBps(pacingRate), pacingBytes)
	return s.pacingBytes
}

func IntoBps(bytesPerSecond float32) string {
	return fmt.Sprintf("%.2fkbps", bytesPerSecond/1024.0*8.0)
}

func (s *PacingSender) PacingBytes() int64 {
	return s.pacingBytes
}

func (s *PacingSender) PacingSend(size int64) bool {
	s.pacingBytes -= size
	if s.pacingBytes <= 0 {
		return false
	}
	return true
}
