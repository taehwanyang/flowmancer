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
	Pid      uint32
	Tgid     uint32
	UID      uint32
	NetnsIno uint32

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
	buf := b[:want]
	r := binary.LittleEndian

	ev.TsNS = r.Uint64(buf[0:8])
	ev.Pid = r.Uint32(buf[8:12])
	ev.Tgid = r.Uint32(buf[12:16])
	ev.UID = r.Uint32(buf[16:20])
	ev.NetnsIno = r.Uint32(buf[20:24])
	ev.Family = r.Uint16(buf[24:26])
	ev.Dport = r.Uint16(buf[26:28])
	ev.Sport = r.Uint16(buf[28:30])

	copy(ev.Comm[:], buf[32:48])
	copy(ev.SaddrV6[:], buf[48:64])
	copy(ev.DaddrV6[:], buf[64:80])

	return ev, nil
}
