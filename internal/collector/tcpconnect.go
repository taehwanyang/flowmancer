package collector

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
	"github.com/cilium/ebpf/rlimit"

	"github.com/taehwanyang/flowmancer/internal/ebpfgen"
	"github.com/taehwanyang/flowmancer/internal/model"
)

type TCPConnectCollector struct {
	objects   ebpfgen.FlowObjects
	links     []link.Link
	ring      *ringbuf.Reader
	onEvent   func(model.TCPConnectEvent)
	onDropped func(error)
}

func NewTCPConnectCollector(
	onEvent func(model.TCPConnectEvent),
	onDropped func(error),
) *TCPConnectCollector {
	return &TCPConnectCollector{
		onEvent:   onEvent,
		onDropped: onDropped,
	}
}

func (c *TCPConnectCollector) Start(ctx context.Context) error {
	if err := rlimit.RemoveMemlock(); err != nil {
		return fmt.Errorf("remove memlock: %w", err)
	}

	if err := ebpfgen.LoadFlowObjects(&c.objects, nil); err != nil {
		return fmt.Errorf("load flow objects: %w", err)
	}

	kp4, err := link.Kprobe("tcp_v4_connect", c.objects.HandleTcpV4Connect, nil)
	if err != nil {
		c.Close()
		return fmt.Errorf("attach tcp_v4_connect: %w", err)
	}
	log.Println("attached kprobe: tcp_v4_connect")
	c.links = append(c.links, kp4)

	kp6, err := link.Kprobe("tcp_v6_connect", c.objects.HandleTcpV6Connect, nil)
	if err != nil {
		c.Close()
		return fmt.Errorf("attach tcp_v6_connect: %w", err)
	}
	log.Println("attached kprobe: tcp_v6_connect")
	c.links = append(c.links, kp6)

	rd, err := ringbuf.NewReader(c.objects.Events)
	if err != nil {
		c.Close()
		return fmt.Errorf("open ringbuf: %w", err)
	}
	log.Println("opened ringbuf reader")
	c.ring = rd

	go func() {
		<-ctx.Done()
		_ = c.Close()
	}()

	go c.readLoop()
	return nil
}

func (c *TCPConnectCollector) readLoop() {
	for {
		rec, err := c.ring.Read()
		if err != nil {
			if errors.Is(err, ringbuf.ErrClosed) {
				return
			}
			if c.onDropped != nil {
				c.onDropped(err)
			}
			continue
		}

		ev, err := model.DecodeTCPConnectEvent(rec.RawSample)
		if err != nil {
			if c.onDropped != nil {
				c.onDropped(err)
			}
			continue
		}

		if c.onEvent != nil {
			c.onEvent(ev)
		}
	}
}

func (c *TCPConnectCollector) Close() error {
	var first error

	if c.ring != nil {
		if err := c.ring.Close(); err != nil && first == nil {
			first = err
		}
		c.ring = nil
	}

	for _, l := range c.links {
		if err := l.Close(); err != nil && first == nil {
			first = err
		}
	}
	c.links = nil

	if err := c.objects.Close(); err != nil && first == nil {
		first = err
	}

	return first
}

func ExampleLogEvent(ev model.TCPConnectEvent) {
	log.Printf(
		"ts=%s comm=%s pid=%d tgid=%d netns=%d dst=%s:%d family=%d",
		ev.Time().Format("15:04:05.000"),
		ev.CommString(),
		ev.Pid,
		ev.Tgid,
		ev.NetnsIno,
		ev.DstIP(),
		ev.Dport,
		ev.Family,
	)
}
