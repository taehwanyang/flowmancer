package dns

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
	"github.com/cilium/ebpf/rlimit"

	"github.com/taehwanyang/flowmancer/internal/ebpfgen"
)

type Event struct {
	TsNS       uint64
	NetnsIno   uint32
	Family     uint16
	Sport      uint16
	Dport      uint16
	PayloadLen uint16
	SaddrV6    [16]byte
	DaddrV6    [16]byte
	Payload    [512]byte
}

type Collector struct {
	objects ebpfgen.DNSObjects
	ring    *ringbuf.Reader
	link    link.Link
	onResp  func(*ParsedResponse)

	closeOnce sync.Once
}

func NewCollector(onResp func(*ParsedResponse)) *Collector {
	return &Collector{onResp: onResp}
}

func (c *Collector) Start(ctx context.Context, iface *net.Interface) error {
	log.Printf("[dns] selected interface: %s (ifindex=%d)", iface.Name, iface.Index)
	return c.StartOnInterface(ctx, iface)
}

func (c *Collector) StartOnInterface(ctx context.Context, iface *net.Interface) error {
	if err := rlimit.RemoveMemlock(); err != nil {
		return fmt.Errorf("remove memlock: %w", err)
	}

	if err := ebpfgen.LoadDNSObjects(&c.objects, nil); err != nil {
		return fmt.Errorf("load dns objects: %w", err)
	}

	rd, err := ringbuf.NewReader(c.objects.DnsEvents)
	if err != nil {
		_ = c.objects.Close()
		return fmt.Errorf("open dns ringbuf: %w", err)
	}
	c.ring = rd

	l, err := attachTCIngress(c.objects.HandleDnsTc, iface.Index)
	if err != nil {
		_ = c.ring.Close()
		_ = c.objects.Close()
		return fmt.Errorf("attach tc ingress on %s(%d): %w", iface.Name, iface.Index, err)
	}
	c.link = l

	log.Printf("[dns] attached tc ingress on %s (ifindex=%d)", iface.Name, iface.Index)

	go func() {
		<-ctx.Done()
		_ = c.Close()
	}()

	go c.readLoop()
	return nil
}

func attachTCIngress(prog *ebpf.Program, ifIndex int) (link.Link, error) {
	if prog == nil {
		return nil, errors.New("nil tc program")
	}

	l, err := link.AttachTCX(link.TCXOptions{
		Interface: ifIndex,
		Program:   prog,
		Attach:    ebpf.AttachTCXEgress,
	})
	if err != nil {
		return nil, err
	}

	return l, nil
}

func (c *Collector) readLoop() {
	for {
		rec, err := c.ring.Read()
		if err != nil {
			if errors.Is(err, ringbuf.ErrClosed) {
				return
			}
			log.Printf("dns collector read error: %v", err)
			continue
		}

		ev, err := decodeEvent(rec.RawSample)
		if err != nil {
			log.Printf("dns decode error: %v", err)
			continue
		}

		if ev.PayloadLen == 0 || int(ev.PayloadLen) > len(ev.Payload) {
			log.Printf("[dns] skip invalid payload_len=%d", ev.PayloadLen)
			continue
		}

		resp, err := ParseResponse(ev.Payload[:ev.PayloadLen])
		if err != nil {
			log.Printf("[dns] parse failed payload_len=%d sport=%d dport=%d err=%v",
				ev.PayloadLen,
				ev.Sport,
				ev.Dport,
				err,
			)
			continue
		}

		log.Printf("[dns] domain=%s ips=%v ttl=%d", resp.Domain, resp.IPs, resp.TTL)

		if c.onResp != nil {
			c.onResp(resp)
		}
	}
}

func (c *Collector) Close() error {
	var first error

	c.closeOnce.Do(func() {
		if c.ring != nil {
			if err := c.ring.Close(); err != nil && first == nil {
				first = err
			}
		}
		if c.link != nil {
			if err := c.link.Close(); err != nil && first == nil {
				first = err
			}
		}
		if err := c.objects.Close(); err != nil && first == nil {
			first = err
		}
	})

	return first
}
