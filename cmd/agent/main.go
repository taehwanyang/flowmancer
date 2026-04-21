package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/taehwanyang/flowmancer/internal/aggregator"
	"github.com/taehwanyang/flowmancer/internal/dns"
	"github.com/taehwanyang/flowmancer/internal/k8smeta"
	"github.com/taehwanyang/flowmancer/internal/netutil"
	"github.com/taehwanyang/flowmancer/internal/tcp"
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

	srcResolver := k8smeta.NewSrcResolver(clientset, nodeName)
	dstResolver := k8smeta.NewDstResolver(clientset)

	if err := srcResolver.Start(ctx); err != nil {
		log.Fatalf("start src resolver: %v", err)
	}
	if err := dstResolver.Start(ctx); err != nil {
		log.Fatalf("start dst resolver: %v", err)
	}

	dnsCache := dns.NewCache()
	agg := aggregator.NewWorkloadBaselineAggregator()

	dnsRespHandler := NewDNSRespHandler(dnsCache)
	dnsCollector := dns.NewDNSRespCollector(dnsRespHandler.Handle)

	tcpConnectEventHandler := NewTCPConnectEventHandler(srcResolver, dnsCache, dstResolver, agg)
	tcpCollector := tcp.NewTCPConnectCollector(
		tcpConnectEventHandler.Handle,
		func(err error) {
			log.Printf("collector error: %v", err)
		},
	)

	iface, err := netutil.DetectBridgeInterface()
	if err != nil {
		log.Fatalf("find bridge interface: %v", err)
	}
	if err := dnsCollector.Start(ctx, iface); err != nil {
		log.Fatalf("start dns collector: %v", err)
	}
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
