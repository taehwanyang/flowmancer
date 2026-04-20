package dns

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strings"
)

type ParsedResponse struct {
	Domain string
	IPs    []net.IP
	TTL    uint32
}

func ParseResponse(msg []byte) (*ParsedResponse, error) {
	if len(msg) < 12 {
		return nil, errors.New("short dns message")
	}

	flags := binary.BigEndian.Uint16(msg[2:4])
	qdcount := int(binary.BigEndian.Uint16(msg[4:6]))
	ancount := int(binary.BigEndian.Uint16(msg[6:8]))

	// QR bit = response
	if flags&0x8000 == 0 {
		return nil, errors.New("not a response")
	}

	offset := 12

	var qname string
	for i := 0; i < qdcount; i++ {
		name, next, err := readName(msg, offset)
		if err != nil {
			return nil, fmt.Errorf("read qname: %w", err)
		}
		qname = name

		offset = next + 4 // qtype + qclass
		if offset > len(msg) {
			return nil, errors.New("question out of bounds")
		}
	}

	var ips []net.IP
	var ttl uint32
	var ttlSet bool

	for i := 0; i < ancount; i++ {
		_, next, err := readName(msg, offset)
		if err != nil {
			return nil, fmt.Errorf("read aname: %w", err)
		}
		offset = next

		if offset+10 > len(msg) {
			return nil, errors.New("answer header out of bounds")
		}

		rtype := binary.BigEndian.Uint16(msg[offset : offset+2])
		offset += 2

		_ = binary.BigEndian.Uint16(msg[offset : offset+2]) // class
		offset += 2

		recordTTL := binary.BigEndian.Uint32(msg[offset : offset+4])
		offset += 4

		rdlen := int(binary.BigEndian.Uint16(msg[offset : offset+2]))
		offset += 2

		if offset+rdlen > len(msg) {
			return nil, errors.New("rdata out of bounds")
		}

		if rtype == 1 && rdlen == 4 { // A
			ip := net.IPv4(msg[offset], msg[offset+1], msg[offset+2], msg[offset+3]).To4()
			ips = append(ips, ip)

			if !ttlSet || recordTTL < ttl {
				ttl = recordTTL
				ttlSet = true
			}
		}

		offset += rdlen
	}

	if qname == "" || len(ips) == 0 {
		return nil, errors.New("no qname or no A records")
	}

	if !ttlSet || ttl == 0 {
		ttl = 30
	}

	return &ParsedResponse{
		Domain: qname,
		IPs:    ips,
		TTL:    ttl,
	}, nil
}

func readName(msg []byte, offset int) (string, int, error) {
	var labels []string
	start := offset
	jumped := false
	seen := 0

	for {
		if offset >= len(msg) {
			return "", 0, errors.New("name out of bounds")
		}
		if seen > len(msg) {
			return "", 0, errors.New("name compression loop")
		}
		seen++

		l := int(msg[offset])

		if l == 0 {
			offset++
			break
		}

		if l&0xC0 == 0xC0 {
			if offset+1 >= len(msg) {
				return "", 0, errors.New("bad compression pointer")
			}
			ptr := int(binary.BigEndian.Uint16(msg[offset:offset+2]) & 0x3FFF)
			if ptr >= len(msg) {
				return "", 0, errors.New("compression ptr out of bounds")
			}
			if !jumped {
				start = offset + 2
			}
			offset = ptr
			jumped = true
			continue
		}

		offset++
		if offset+l > len(msg) {
			return "", 0, errors.New("label out of bounds")
		}
		labels = append(labels, string(msg[offset:offset+l]))
		offset += l
	}

	if !jumped {
		start = offset
	}

	return strings.Join(labels, "."), start, nil
}
