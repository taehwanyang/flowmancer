package aggregator

import (
	"net"
	"sort"

	"github.com/taehwanyang/flowmancer/internal/model"
)

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

func sortClosedWindows(out []ClosedWindow) {
	sort.Slice(out, func(i, j int) bool {
		a := out[i]
		b := out[j]

		if !a.WindowStart.Equal(b.WindowStart) {
			return a.WindowStart.Before(b.WindowStart)
		}
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

func SubjectStringFromKey(key WorkloadFlowKey) string {
	if key.Namespace != "" && key.WorkloadName != "" {
		return key.Namespace + "/" + key.WorkloadKind + "/" + key.WorkloadName
	}
	return "netns=" + itoa32(key.NetnsIno) + " comm=" + key.Comm
}
