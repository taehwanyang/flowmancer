package aggregator

import (
	"sync"
	"time"
)

type WindowFlowAggregate struct {
	Key         WorkloadFlowKey
	WindowStart time.Time
	WindowEnd   time.Time

	Count        uint64
	SuccessCount uint64
	FailCount    uint64

	FirstSeen time.Time
	LastSeen  time.Time

	TotalDuration time.Duration
}

type ClosedWindow struct {
	Key         WorkloadFlowKey
	WindowStart time.Time
	WindowEnd   time.Time

	Count        uint64
	SuccessCount uint64
	FailCount    uint64

	FirstSeen time.Time
	LastSeen  time.Time

	TotalDuration time.Duration
}

func (w ClosedWindow) SuccessRatio() float64 {
	if w.Count == 0 {
		return 0
	}
	return float64(w.SuccessCount) / float64(w.Count)
}

func (w ClosedWindow) AvgDuration() time.Duration {
	if w.Count == 0 {
		return 0
	}
	return time.Duration(int64(w.TotalDuration) / int64(w.Count))
}

type WorkloadWindowAggregator struct {
	mu         sync.Mutex
	windowSize time.Duration
	flows      map[WorkloadFlowKey]*WindowFlowAggregate
}

func NewWorkloadWindowAggregator(windowSize time.Duration) *WorkloadWindowAggregator {
	if windowSize <= 0 {
		windowSize = 5 * time.Minute
	}

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

	key := BuildWorkloadKey(in)
	if key.DstPort == 0 {
		return
	}
	if key.Dst == "" {
		return
	}

	dur := ev.Duration()

	ts := in.ObservedAt
	ws := ts.Truncate(a.windowSize)
	we := ws.Add(a.windowSize)

	a.mu.Lock()
	defer a.mu.Unlock()

	agg, ok := a.flows[key]
	if !ok || !agg.WindowStart.Equal(ws) {
		agg = &WindowFlowAggregate{
			Key:         key,
			WindowStart: ws,
			WindowEnd:   we,
			FirstSeen:   ts,
			LastSeen:    ts,
		}
		a.flows[key] = agg
	}

	agg.Count++
	if ev.Ret == 0 {
		agg.SuccessCount++
	} else {
		agg.FailCount++
	}

	if ts.Before(agg.FirstSeen) {
		agg.FirstSeen = ts
	}
	if ts.After(agg.LastSeen) {
		agg.LastSeen = ts
	}

	agg.TotalDuration += dur
}

func (a *WorkloadWindowAggregator) PopExpired(now time.Time) []ClosedWindow {
	a.mu.Lock()
	defer a.mu.Unlock()

	out := make([]ClosedWindow, 0)

	for key, agg := range a.flows {
		if agg.WindowEnd.After(now) {
			continue
		}

		out = append(out, ClosedWindow{
			Key:           agg.Key,
			WindowStart:   agg.WindowStart,
			WindowEnd:     agg.WindowEnd,
			Count:         agg.Count,
			SuccessCount:  agg.SuccessCount,
			FailCount:     agg.FailCount,
			FirstSeen:     agg.FirstSeen,
			LastSeen:      agg.LastSeen,
			TotalDuration: agg.TotalDuration,
		})

		delete(a.flows, key)
	}

	sortClosedWindows(out)
	return out
}

func (a *WorkloadWindowAggregator) SnapshotOpenWindows() []WindowFlowAggregate {
	a.mu.Lock()
	defer a.mu.Unlock()

	out := make([]WindowFlowAggregate, 0, len(a.flows))
	for _, agg := range a.flows {
		out = append(out, WindowFlowAggregate{
			Key:           agg.Key,
			WindowStart:   agg.WindowStart,
			WindowEnd:     agg.WindowEnd,
			Count:         agg.Count,
			SuccessCount:  agg.SuccessCount,
			FailCount:     agg.FailCount,
			FirstSeen:     agg.FirstSeen,
			LastSeen:      agg.LastSeen,
			TotalDuration: agg.TotalDuration,
		})
	}

	return out
}

func (a *WorkloadWindowAggregator) WindowSize() time.Duration {
	return a.windowSize
}

func (a *WorkloadWindowAggregator) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.flows = make(map[WorkloadFlowKey]*WindowFlowAggregate)
}
