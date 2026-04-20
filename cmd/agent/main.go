package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/taehwanyang/flowmancer/internal/aggregator"
	"github.com/taehwanyang/flowmancer/internal/collector"
	"github.com/taehwanyang/flowmancer/internal/dns"
	"github.com/taehwanyang/flowmancer/internal/k8smeta"
	"github.com/taehwanyang/flowmancer/internal/model"
	"github.com/taehwanyang/flowmancer/internal/netutil"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	nodeName := os.Getenv("MY_NODE_NAME")
	if nodeName == "" {
		log.Println("[warn] MY_NODE_NAME not set, resolving all pods (dev mode)")
	} else {
		log.Printf("[info] running on node=%s", nodeName)
	}

	clientset, err := k8smeta.NewKubernetesClient()
	if err != nil {
		log.Fatalf("new kubernetes client: %v", err)
	}

	resolver := k8smeta.NewResolver(clientset, nodeName)
	if err := resolver.Start(ctx); err != nil {
		log.Fatalf("start resolver: %v", err)
	}

	dnsCache := dns.NewCache()

	iface, err := netutil.DetectBridgeInterface()
	if err != nil {
		log.Fatalf("find bridge interface: %v", err)
	}

	dnsCollector := dns.NewCollector(func(resp *dns.ParsedResponse) {
		dnsCache.Add(resp.Domain, resp.IPs, resp.TTL)
		log.Printf("[dns] domain=%s ips=%v ttl=%d", resp.Domain, resp.IPs, resp.TTL)
	})

	if err := dnsCollector.Start(ctx, iface); err != nil {
		log.Fatalf("start dns collector: %v", err)
	}

	agg := aggregator.NewWorkloadBaselineAggregator()

	tcpCollector := collector.NewTCPConnectCollector(
		func(ev model.TCPConnectEvent) {
			var pod *k8smeta.PodMetadata

			if resolved, ok := resolver.ResolveNetns(ev.NetnsIno); ok {
				pod = &resolved
				log.Printf(
					"[resolved] netns=%d -> %s/%s workload=%s/%s",
					ev.NetnsIno,
					resolved.Namespace,
					resolved.PodName,
					resolved.WorkloadKind,
					resolved.WorkloadName,
				)
			} else {
				log.Printf("[resolved] netns=%d -> <unresolved>", ev.NetnsIno)
			}

			var domain string
			if d, ok := dnsCache.Lookup(ev.DstIP()); ok {
				domain = d
				log.Printf("[dns lookup] hit dst=%s domain=%s", ev.DstIP().String(), domain)
			} else {
				log.Printf("[dns lookup] miss dst=%s", ev.DstIP().String())
			}

			agg.Add(aggregator.ResolvedFlow{
				Event:  ev,
				Pod:    pod,
				Domain: domain,
			})

			collector.ExampleLogEvent(ev)
		},
		func(err error) {
			log.Printf("collector error: %v", err)
		},
	)

	if err := tcpCollector.Start(ctx); err != nil {
		log.Fatalf("start tcp collector: %v", err)
	}

	log.Println("flowmancer tcp connect collector started")

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				printSnapshotTopN(agg, 10)
			}
		}
	}()

	<-ctx.Done()

	log.Println("final workload baseline candidates:")
	printBaselineCandidatesAuto(agg)

	log.Println("flowmancer tcp connect collector stopped")
}
