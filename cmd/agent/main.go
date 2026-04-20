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

	// source resolver (netns -> pod)
	resolver := k8smeta.NewResolver(clientset, nodeName)
	if err := resolver.Start(ctx); err != nil {
		log.Fatalf("start resolver: %v", err)
	}

	// destination resolver (dst IP -> Service/Pod/Workload)
	dstResolver := k8smeta.NewDstResolver(clientset)
	if err := dstResolver.Refresh(ctx); err != nil {
		log.Printf("[warn] initial dst resolver refresh failed: %v", err)
	}

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := dstResolver.Refresh(ctx); err != nil {
					log.Printf("refresh dst resolver failed: %v", err)
				}
			}
		}
	}()

	dnsCache := dns.NewCache()

	iface, err := netutil.DetectBridgeInterface()
	if err != nil {
		log.Fatalf("find bridge interface: %v", err)
	}

	dnsCollector := dns.NewCollector(func(resp *dns.ParsedResponse) {
		if resp == nil || resp.Domain == "" || len(resp.IPs) == 0 {
			return
		}

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

			dstIP := ev.DstIP()
			if v4 := dstIP.To4(); v4 != nil {
				dstIP = v4
			}

			var domain string
			if d, ok := dnsCache.Lookup(dstIP); ok {
				domain = d
				log.Printf("[dns lookup] hit dst=%s domain=%s", dstIP.String(), domain)
			} else {
				log.Printf("[dns lookup] miss dst=%s", dstIP.String())
			}

			var dstK8sName string
			if resolvedDst, ok := dstResolver.ResolveDstIP(dstIP); ok {
				dstK8sName = resolvedDst.Name
				log.Printf("[k8s dst] hit dst=%s name=%s", dstIP.String(), dstK8sName)
			} else {
				log.Printf("[k8s dst] miss dst=%s", dstIP.String())
			}

			agg.Add(aggregator.ResolvedFlow{
				Event:      ev,
				Pod:        pod,
				Domain:     domain,
				DstK8sName: dstK8sName,
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

	defer func() {
		if err := dnsCollector.Close(); err != nil {
			log.Printf("close dns collector: %v", err)
		}
		if err := tcpCollector.Close(); err != nil {
			log.Printf("close tcp collector: %v", err)
		}
	}()

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
