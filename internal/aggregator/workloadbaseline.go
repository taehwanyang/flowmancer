package aggregator

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/taehwanyang/flowmancer/internal/k8smeta"
	"github.com/taehwanyang/flowmancer/internal/model"
)

type ResolvedFlow struct {
	Event  model.TCPConnectEvent
	Pod    *k8smeta.PodMetadata
	Domain string
}

type WorkloadFlowKey struct {
	Namespace    string
	WorkloadKind string
	WorkloadName string

	NetnsIno uint32
	Comm     string

	Family    uint16
	DstDomain string
	DstIP     string
	DstPort   uint16
}

type WorkloadFlowAggregate struct {
	Key WorkloadFlowKey

	Count        uint64
	SuccessCount uint64
	FailCount    uint64

	FirstSeen time.Time
	LastSeen  time.Time

	TotalDuration time.Duration
}

func (f WorkloadFlowAggregate) SuccessRatio() float64 {
	if f.Count == 0 {
		return 0
	}
	return float64(f.SuccessCount) / float64(f.Count)
}

func (f WorkloadFlowAggregate) AvgDuration() time.Duration {
	if f.Count == 0 {
		return 0
	}
	return time.Duration(int64(f.TotalDuration) / int64(f.Count))
}

func (f WorkloadFlowAggregate) SubjectString() string {
	if f.Key.Namespace != "" && f.Key.WorkloadName != "" {
		return f.Key.Namespace + "/" + f.Key.WorkloadKind + "/" + f.Key.WorkloadName
	}
	return "netns=" + itoa32(f.Key.NetnsIno) + " comm=" + f.Key.Comm
}

func (f WorkloadFlowAggregate) DestinationString() string {
	if f.Key.DstDomain != "" {
		return f.Key.DstDomain
	}
	return f.Key.DstIP
}

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
	if key.DstDomain == "" && key.DstIP == "" {
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

func buildWorkloadKey(in ResolvedFlow) WorkloadFlowKey {
	ev := in.Event

	key := WorkloadFlowKey{
		Family:  ev.Family,
		DstPort: ev.Dport,
	}

	if in.Domain != "" {
		key.DstDomain = normalizeDomain(in.Domain)
	} else {
		key.DstIP = ipString(ev.DstIP())
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

func normalizeDomain(d string) string {
	d = strings.TrimSpace(strings.ToLower(d))
	d = strings.TrimSuffix(d, ".")
	if strings.HasPrefix(d, "www.") {
		return d[4:]
	}
	return d
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

func sortWorkloadAggregates(out []WorkloadFlowAggregate) {
	sort.Slice(out, func(i, j int) bool {
		a := out[i]
		b := out[j]

		if a.Count != b.Count {
			return a.Count > b.Count
		}
		if a.Key.Namespace != b.Key.Namespace {
			return a.Key.Namespace < b.Key.Namespace
		}
		if a.Key.WorkloadKind != b.Key.WorkloadKind {
			return a.Key.WorkloadKind < b.Key.WorkloadKind
		}
		if a.Key.WorkloadName != b.Key.WorkloadName {
			return a.Key.WorkloadName < b.Key.WorkloadName
		}
		if a.Key.NetnsIno != b.Key.NetnsIno {
			return a.Key.NetnsIno < b.Key.NetnsIno
		}
		if a.Key.Comm != b.Key.Comm {
			return a.Key.Comm < b.Key.Comm
		}

		adst := a.DestinationString()
		bdst := b.DestinationString()
		if adst != bdst {
			return adst < bdst
		}

		return a.Key.DstPort < b.Key.DstPort
	})
}

func itoa32(v uint32) string {
	if v == 0 {
		return "0"
	}
	var buf [10]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
