package aggregator

import "sync"

type WorkloadBaselineAggregator struct {
	mu    sync.RWMutex
	flows map[WorkloadFlowKey]*WorkloadFlowAggregate
}

func NewWorkloadBaselineAggregator() *WorkloadBaselineAggregator {
	return &WorkloadBaselineAggregator{
		flows: make(map[WorkloadFlowKey]*WorkloadFlowAggregate),
	}
}

func (a *WorkloadBaselineAggregator) Add(in ResolvedFlow) {
	ev := in.Event
	if isNoise(ev) {
		return
	}

	key := buildWorkloadKey(in)
	if key.DstPort == 0 {
		return
	}
	if key.Dst == "" {
		return
	}

	ts := ev.Time()
	dur := ev.Duration()

	a.mu.Lock()
	defer a.mu.Unlock()

	agg, ok := a.flows[key]
	if !ok {
		agg = &WorkloadFlowAggregate{
			Key:       key,
			FirstSeen: ts,
			LastSeen:  ts,
			DaysSeen:  make(map[string]uint64),
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

	dayKey := ts.Format("2006-01-02")
	agg.DaysSeen[dayKey]++
}

func (a *WorkloadBaselineAggregator) Get(key WorkloadFlowKey) (WorkloadFlowAggregate, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	agg, ok := a.flows[key]
	if !ok {
		return WorkloadFlowAggregate{}, false
	}
	return *agg, true
}

func (a *WorkloadBaselineAggregator) AppendWindowSample(key WorkloadFlowKey, count uint64, maxSamples int) {
	a.mu.Lock()
	defer a.mu.Unlock()

	agg, ok := a.flows[key]
	if !ok {
		return
	}

	agg.WindowCounts = append(agg.WindowCounts, count)
	if maxSamples > 0 && len(agg.WindowCounts) > maxSamples {
		agg.WindowCounts = agg.WindowCounts[len(agg.WindowCounts)-maxSamples:]
	}
}

func (a *WorkloadBaselineAggregator) Snapshot() []WorkloadFlowAggregate {
	a.mu.RLock()
	defer a.mu.RUnlock()

	out := make([]WorkloadFlowAggregate, 0, len(a.flows))
	for _, agg := range a.flows {
		out = append(out, *agg)
	}

	sortWorkloadAggregates(out)
	return out
}

func (a *WorkloadBaselineAggregator) SnapshotTopN(n int) []WorkloadFlowAggregate {
	all := a.Snapshot()
	if n <= 0 || n >= len(all) {
		return all
	}
	return all[:n]
}

func (a *WorkloadBaselineAggregator) BaselineCandidatesAuto() ([]WorkloadFlowAggregate, uint64) {
	all := a.Snapshot()

	var total uint64
	for _, agg := range all {
		total += agg.Count
	}

	minCount := autoMinCount(total)

	out := make([]WorkloadFlowAggregate, 0, len(all))
	for _, agg := range all {
		if agg.Count < minCount {
			continue
		}
		if agg.SuccessRatio() < 0.8 {
			continue
		}
		out = append(out, agg)
	}

	return out, minCount
}
