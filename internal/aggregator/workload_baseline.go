package aggregator

import (
	"sort"
	"strings"
	"time"

	"github.com/taehwanyang/flowmancer/internal/k8smeta"
	"github.com/taehwanyang/flowmancer/internal/model"
)

type ResolvedFlow struct {
	Event      model.TCPConnectEvent
	Pod        *k8smeta.PodMetadata
	Domain     string
	DstK8sName string
}

type WorkloadFlowKey struct {
	Namespace    string
	WorkloadKind string
	WorkloadName string

	NetnsIno uint32
	Comm     string

	Family  uint16
	Dst     string
	DstPort uint16
}

type WorkloadFlowAggregate struct {
	Key WorkloadFlowKey

	Count        uint64
	SuccessCount uint64
	FailCount    uint64

	FirstSeen time.Time
	LastSeen  time.Time

	TotalDuration time.Duration

	DaysSeen     map[string]uint64
	WindowCounts []uint64
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
	return f.Key.Dst
}

func buildWorkloadKey(in ResolvedFlow) WorkloadFlowKey {
	ev := in.Event

	key := WorkloadFlowKey{
		Family:  ev.Family,
		DstPort: ev.Dport,
	}

	switch {
	case in.Domain != "":
		key.Dst = normalizeDomain(in.Domain)
	case in.DstK8sName != "":
		key.Dst = in.DstK8sName
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

func normalizeDomain(d string) string {
	d = strings.TrimSpace(strings.ToLower(d))
	d = strings.TrimSuffix(d, ".")
	if strings.HasPrefix(d, "www.") {
		return d[4:]
	}
	return d
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
		if a.Key.Dst != b.Key.Dst {
			return a.Key.Dst < b.Key.Dst
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
