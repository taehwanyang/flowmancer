package dns

import (
	"encoding/binary"
	"fmt"
)

func decodeEvent(b []byte) (Event, error) {
	var ev Event
	want := binary.Size(ev)
	if len(b) < want {
		return ev, fmt.Errorf("short sample: got=%d want=%d", len(b), want)
	}
	r := binary.LittleEndian

	ev.TsNS = r.Uint64(b[0:8])
	ev.Pid = r.Uint32(b[8:12])
	ev.Tgid = r.Uint32(b[12:16])
	ev.UID = r.Uint32(b[16:20])
	ev.NetnsIno = r.Uint32(b[20:24])
	ev.Family = r.Uint16(b[24:26])
	ev.Sport = r.Uint16(b[26:28])
	ev.Dport = r.Uint16(b[28:30])
	ev.PayloadLen = r.Uint16(b[30:32])

	copy(ev.Comm[:], b[32:48])
	copy(ev.SaddrV6[:], b[48:64])
	copy(ev.DaddrV6[:], b[64:80])
	copy(ev.Payload[:], b[80:592])

	return ev, nil
}
