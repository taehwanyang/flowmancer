package dns

import (
	"encoding/binary"
	"fmt"
)

func decodeDNSRespEvent(b []byte) (DNSRespEvent, error) {
	var ev DNSRespEvent
	want := binary.Size(ev)
	if len(b) < want {
		return ev, fmt.Errorf("short sample: got=%d want=%d", len(b), want)
	}

	r := binary.LittleEndian

	ev.TsNS = r.Uint64(b[0:8])
	ev.NetnsIno = r.Uint32(b[8:12])

	ev.Family = r.Uint16(b[12:14])
	ev.Sport = r.Uint16(b[14:16])
	ev.Dport = r.Uint16(b[16:18])
	ev.PayloadLen = r.Uint16(b[18:20])

	copy(ev.SaddrV6[:], b[20:36])
	copy(ev.DaddrV6[:], b[36:52])
	copy(ev.Payload[:], b[52:564])

	if int(ev.PayloadLen) > len(ev.Payload) {
		return ev, fmt.Errorf("invalid payload_len: %d", ev.PayloadLen)
	}

	return ev, nil
}
