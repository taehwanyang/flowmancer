package aggregator

import (
	"net"

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
