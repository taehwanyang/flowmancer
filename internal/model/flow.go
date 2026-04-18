package model

import (
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

const (
	AFInet  = 2
	AFInet6 = 10
)

type TCPConnectEvent struct {
	TsNS     uint64
	EndTsNS  uint64
	Pid      uint32
	Tgid     uint32
	UID      uint32
	NetnsIno uint32

	Ret    int32
	Family uint16
	Dport  uint16
	Sport  uint16
	_      uint16

	Comm    [16]byte
	SaddrV6 [16]byte
	DaddrV6 [16]byte
}

func (e TCPConnectEvent) Time() time.Time {
	return time.Unix(0, int64(e.TsNS))
}

func (e TCPConnectEvent) Duration() time.Duration {
	if e.EndTsNS <= e.TsNS {
		return 0
	}
	return time.Duration(e.EndTsNS - e.TsNS)
}

func (e TCPConnectEvent) CommString() string {
	n := 0
	for ; n < len(e.Comm); n++ {
		if e.Comm[n] == 0 {
			break
		}
	}
	return string(e.Comm[:n])
}

func (e TCPConnectEvent) SrcIP() net.IP {
	switch e.Family {
	case AFInet:
		return net.IP(e.SaddrV6[:4])
	case AFInet6:
		return net.IP(e.SaddrV6[:])
	default:
		return nil
	}
}

func (e TCPConnectEvent) DstIP() net.IP {
	switch e.Family {
	case AFInet:
		return net.IP(e.DaddrV6[:4])
	case AFInet6:
		return net.IP(e.DaddrV6[:])
	default:
		return nil
	}
}

func DecodeTCPConnectEvent(b []byte) (TCPConnectEvent, error) {
	var ev TCPConnectEvent
	want := binary.Size(ev)
	if len(b) < want {
		return ev, fmt.Errorf("short sample: got=%d want>=%d", len(b), want)
	}

	r := binary.LittleEndian
	buf := b[:want]

	ev.TsNS = r.Uint64(buf[0:8])
	ev.EndTsNS = r.Uint64(buf[8:16])
	ev.Pid = r.Uint32(buf[16:20])
	ev.Tgid = r.Uint32(buf[20:24])
	ev.UID = r.Uint32(buf[24:28])
	ev.NetnsIno = r.Uint32(buf[28:32])

	ev.Ret = int32(r.Uint32(buf[32:36]))
	ev.Family = r.Uint16(buf[36:38])
	ev.Dport = r.Uint16(buf[38:40])
	ev.Sport = r.Uint16(buf[40:42])

	copy(ev.Comm[:], buf[44:60])
	copy(ev.SaddrV6[:], buf[60:76])
	copy(ev.DaddrV6[:], buf[76:92])

	return ev, nil
}
