package model

import (
	"time"

	"golang.org/x/sys/unix"
)

type MonotonicClockConverter struct {
	bootTime time.Time
}

func NewMonotonicClockConverter() (*MonotonicClockConverter, error) {
	monoNowNS, err := currentMonotonicNS()
	if err != nil {
		return nil, err
	}

	return &MonotonicClockConverter{
		bootTime: time.Now().Add(-time.Duration(monoNowNS)),
	}, nil
}

func (c *MonotonicClockConverter) ToTime(tsNS uint64) time.Time {
	return c.bootTime.Add(time.Duration(tsNS))
}

func currentMonotonicNS() (uint64, error) {
	var ts unix.Timespec
	if err := unix.ClockGettime(unix.CLOCK_MONOTONIC, &ts); err != nil {
		return 0, err
	}
	return uint64(ts.Sec)*1_000_000_000 + uint64(ts.Nsec), nil
}
