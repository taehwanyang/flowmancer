package aggregator

import (
	"sync"
	"time"

	"github.com/taehwanyang/flowmancer/internal/k8smeta"
	"github.com/taehwanyang/flowmancer/internal/model"
)

type BaselineBuilder struct {
	mu    sync.RWMutex
	flows map[WorkloadFlowKey]*WorkloadFlowAggregate
}

func NewBaselineBuilder() *BaselineBuilder {
	return &BaselineBuilder{
		flows: make(map[WorkloadFlowKey]*WorkloadFlowAggregate),
	}
}

func (b *BaselineBuilder) Add(in ResolvedFlow) {
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

	ts := in.ObservedAt
	dur := ev.Duration()

	b.mu.Lock()
	defer b.mu.Unlock()

	agg, ok := b.flows[key]
	if !ok {
		agg = &WorkloadFlowAggregate{
			Key:       key,
			FirstSeen: ts,
			LastSeen:  ts,
			DaysSeen:  make(map[string]uint64),
		}
		b.flows[key] = agg
	}

	if agg.DaysSeen == nil {
		agg.DaysSeen = make(map[string]uint64)
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

func (b *BaselineBuilder) Get(key WorkloadFlowKey) (WorkloadFlowAggregate, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	agg, ok := b.flows[key]
	if !ok {
		return WorkloadFlowAggregate{}, false
	}

	return cloneWorkloadFlowAggregate(*agg), true
}

func (b *BaselineBuilder) AppendWindowSample(key WorkloadFlowKey, count uint64, maxSamples int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	agg, ok := b.flows[key]
	if !ok {
		return
	}

	agg.WindowCounts = append(agg.WindowCounts, count)
	if maxSamples > 0 && len(agg.WindowCounts) > maxSamples {
		agg.WindowCounts = agg.WindowCounts[len(agg.WindowCounts)-maxSamples:]
	}
}

func (b *BaselineBuilder) Snapshot() []WorkloadFlowAggregate {
	b.mu.RLock()
	defer b.mu.RUnlock()

	out := make([]WorkloadFlowAggregate, 0, len(b.flows))
	for _, agg := range b.flows {
		out = append(out, cloneWorkloadFlowAggregate(*agg))
	}

	sortWorkloadAggregates(out)
	return out
}

func (b *BaselineBuilder) SnapshotTopN(n int) []WorkloadFlowAggregate {
	all := b.Snapshot()
	if n <= 0 || n >= len(all) {
		return all
	}
	return all[:n]
}

func (b *BaselineBuilder) BaselineCandidatesAuto() ([]WorkloadFlowAggregate, uint64) {
	all := b.Snapshot()

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

func (b *BaselineBuilder) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.flows)
}

func (b *BaselineBuilder) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.flows = make(map[WorkloadFlowKey]*WorkloadFlowAggregate)
}

func (b *BaselineBuilder) ExportSnapshot() *BaselineSnapshot {
	b.mu.RLock()
	defer b.mu.RUnlock()

	flows := make(map[WorkloadFlowKey]WorkloadFlowAggregate, len(b.flows))
	for key, agg := range b.flows {
		flows[key] = cloneWorkloadFlowAggregate(*agg)
	}

	return &BaselineSnapshot{
		GeneratedAt: time.Now(),
		Flows:       flows,
	}
}

func BuildWorkloadKey(in ResolvedFlow) WorkloadFlowKey {
	ev := in.Event

	key := WorkloadFlowKey{
		Family:  ev.Family,
		DstPort: ev.Dport,
	}

	switch {
	case in.DstK8sName != "":
		key.Dst = in.DstK8sName
	case in.Domain != "":
		key.Dst = normalizeDomain(in.Domain)
	default:
		key.Dst = ipString(ev.DstIP())
	}

	if in.Pod != nil {
		key.Namespace = in.Pod.Namespace
		key.WorkloadKind = in.Pod.WorkloadKind
		key.WorkloadName = in.Pod.WorkloadName
	} else {
		key.NetnsIno = ev.NetnsIno
		key.Comm = ev.CommString()
	}

	return key
}

func NewResolvedFlow(
	ev model.TCPConnectEvent,
	pod *k8smeta.PodMetadata,
	domain string,
	dstK8sName string,
) ResolvedFlow {
	return ResolvedFlow{
		Event:      ev,
		Pod:        pod,
		Domain:     domain,
		DstK8sName: dstK8sName,
	}
}

func cloneWorkloadFlowAggregate(in WorkloadFlowAggregate) WorkloadFlowAggregate {
	out := in

	if in.DaysSeen != nil {
		out.DaysSeen = make(map[string]uint64, len(in.DaysSeen))
		for k, v := range in.DaysSeen {
			out.DaysSeen[k] = v
		}
	}

	if in.WindowCounts != nil {
		out.WindowCounts = append([]uint64(nil), in.WindowCounts...)
	}

	return out
}
