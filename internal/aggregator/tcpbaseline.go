package aggregator

import (
	"log"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/taehwanyang/flowmancer/internal/model"
)

type FlowKey struct {
	NetnsIno uint32
	Comm     string
	Family   uint16
	DstIP    string
	DstPort  uint16
}

type FlowAggregate struct {
	Key FlowKey

	Count        uint64
	SuccessCount uint64
	FailCount    uint64

	FirstSeen time.Time
	LastSeen  time.Time

	TotalDuration time.Duration
}

func (f FlowAggregate) SuccessRatio() float64 {
	if f.Count == 0 {
		return 0
	}
	return float64(f.SuccessCount) / float64(f.Count)
}

func (f FlowAggregate) AvgDuration() time.Duration {
	if f.Count == 0 {
		return 0
	}
	return time.Duration(int64(f.TotalDuration) / int64(f.Count))
}

type TCPBaselineAggregator struct {
	mu    sync.RWMutex
	flows map[FlowKey]*FlowAggregate
}

func NewTCPBaselineAggregator() *TCPBaselineAggregator {
	return &TCPBaselineAggregator{
		flows: make(map[FlowKey]*FlowAggregate),
	}
}

func (a *TCPBaselineAggregator) Add(ev model.TCPConnectEvent) {
	if isNoise(ev) {
		log.Printf("[agg] dropped as noise: comm=%s dst=%s:%d ret=%d family=%d",
			ev.CommString(), ipString(ev.DstIP()), ev.Dport, ev.Ret, ev.Family)
		return
	}

	key := FlowKey{
		NetnsIno: ev.NetnsIno,
		Comm:     ev.CommString(),
		Family:   ev.Family,
		DstIP:    ipString(ev.DstIP()),
		DstPort:  ev.Dport,
	}

	if key.DstIP == "" || key.DstPort == 0 {
		log.Printf("[agg] dropped as empty key: comm=%s dst=%q:%d ret=%d family=%d",
			ev.CommString(), key.DstIP, key.DstPort, ev.Ret, ev.Family)
		return
	}

	ts := ev.Time()
	dur := ev.Duration()

	a.mu.Lock()
	defer a.mu.Unlock()

	agg, ok := a.flows[key]
	if !ok {
		log.Printf("[agg] new flow: netns=%d comm=%s dst=%s:%d",
			key.NetnsIno, key.Comm, key.DstIP, key.DstPort)
		agg = &FlowAggregate{
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

func (a *TCPBaselineAggregator) Snapshot() []FlowAggregate {
	a.mu.RLock()
	defer a.mu.RUnlock()

	out := make([]FlowAggregate, 0, len(a.flows))
	for _, agg := range a.flows {
		out = append(out, *agg)
	}

	sortAggregates(out)
	return out
}

func (a *TCPBaselineAggregator) SnapshotTopN(n int) []FlowAggregate {
	all := a.Snapshot()
	if n <= 0 || n >= len(all) {
		return all
	}
	return all[:n]
}

func (a *TCPBaselineAggregator) BaselineCandidates(minCount uint64) []FlowAggregate {
	all := a.Snapshot()
	out := make([]FlowAggregate, 0, len(all))

	for _, agg := range all {
		if agg.Count < minCount {
			continue
		}
		if agg.SuccessCount == 0 {
			continue
		}
		out = append(out, agg)
	}

	return out
}

// 자동 기준:
// 1) 전체 이벤트 수에 따라 minCount를 완만하게 증가
// 2) 성공률 80% 이상
// 3) 최소 2회 이상은 본 것만 후보
func (a *TCPBaselineAggregator) BaselineCandidatesAuto() ([]FlowAggregate, uint64) {
	all := a.Snapshot()

	var total uint64
	for _, agg := range all {
		total += agg.Count
	}

	minCount := autoMinCount(total)

	out := make([]FlowAggregate, 0, len(all))
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

func autoMinCount(total uint64) uint64 {
	switch {
	case total >= 100:
		return 5
	case total >= 30:
		return 3
	default:
		return 2
	}
}

func sortAggregates(out []FlowAggregate) {
	sort.Slice(out, func(i, j int) bool {
		a := out[i]
		b := out[j]

		if a.Count != b.Count {
			return a.Count > b.Count
		}
		if a.Key.NetnsIno != b.Key.NetnsIno {
			return a.Key.NetnsIno < b.Key.NetnsIno
		}
		if a.Key.Comm != b.Key.Comm {
			return a.Key.Comm < b.Key.Comm
		}
		if a.Key.DstIP != b.Key.DstIP {
			return a.Key.DstIP < b.Key.DstIP
		}
		return a.Key.DstPort < b.Key.DstPort
	})
}

func isNoise(ev model.TCPConnectEvent) bool {
	ip := ev.DstIP()
	if ip == nil {
		return true
	}

	if ev.Dport == 0 {
		return true
	}

	if ip.IsUnspecified() {
		return true
	}

	if ip.IsLoopback() {
		return true
	}

	comm := ev.CommString()
	if comm == "coredns" && (ev.Dport == 8080 || ev.Dport == 8181) {
		return true
	}

	return false
}

func ipString(ip net.IP) string {
	if ip == nil {
		return ""
	}
	return ip.String()
}
