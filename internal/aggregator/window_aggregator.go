package aggregator

import (
	"sync"
	"time"
)

type WindowFlowAggregate struct {
	Key          WorkloadFlowKey
	WindowStart  time.Time
	WindowEnd    time.Time
	Count        uint64
	SuccessCount uint64
	FailCount    uint64
}

type ClosedWindow struct {
	Key          WorkloadFlowKey
	WindowStart  time.Time
	WindowEnd    time.Time
	Count        uint64
	SuccessCount uint64
	FailCount    uint64
}

type WorkloadWindowAggregator struct {
	mu         sync.Mutex
	windowSize time.Duration
	flows      map[WorkloadFlowKey]*WindowFlowAggregate
}

func NewWorkloadWindowAggregator(windowSize time.Duration) *WorkloadWindowAggregator {
	return &WorkloadWindowAggregator{
		windowSize: windowSize,
		flows:      make(map[WorkloadFlowKey]*WindowFlowAggregate),
	}
}

func (a *WorkloadWindowAggregator) Add(in ResolvedFlow) {
	ev := in.Event
	if isNoise(ev) {
		return
	}

	key := buildWorkloadKey(in)
	if key.DstPort == 0 || key.Dst == "" {
		return
	}

	ts := ev.Time()
	windowStart := ts.Truncate(a.windowSize)
	windowEnd := windowStart.Add(a.windowSize)

	a.mu.Lock()
	defer a.mu.Unlock()

	agg, ok := a.flows[key]
	if !ok || !agg.WindowStart.Equal(windowStart) {
		agg = &WindowFlowAggregate{
			Key:         key,
			WindowStart: windowStart,
			WindowEnd:   windowEnd,
		}
		a.flows[key] = agg
	}

	agg.Count++
	if ev.Ret == 0 {
		agg.SuccessCount++
	} else {
		agg.FailCount++
	}
}

func (a *WorkloadWindowAggregator) PopExpired(now time.Time) []ClosedWindow {
	a.mu.Lock()
	defer a.mu.Unlock()

	var out []ClosedWindow

	for key, agg := range a.flows {
		if !agg.WindowEnd.After(now) {
			out = append(out, ClosedWindow{
				Key:          agg.Key,
				WindowStart:  agg.WindowStart,
				WindowEnd:    agg.WindowEnd,
				Count:        agg.Count,
				SuccessCount: agg.SuccessCount,
				FailCount:    agg.FailCount,
			})
			delete(a.flows, key)
		}
	}

	return out
}
