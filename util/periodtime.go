package util

import (
	"time"
)

func NewPeriodTime(period time.Duration, min, max float64) *PeriodTime {
	return &PeriodTime{
		period: period.Nanoseconds(),
		min:    min,
		max:    max,
	}
}

type PeriodTime struct {
	period int64
	min    float64
	max    float64
}

func (p *PeriodTime) Time(t time.Time) float64 {
	return (float64((t.UnixNano())%p.period)/float64(p.period))*(p.max-p.min) + p.min
}
