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
	"github.com/taehwanyang/flowmancer/internal/k8smeta"
	"github.com/taehwanyang/flowmancer/internal/model"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	nodeName := os.Getenv("MY_NODE_NAME")

	clientset, err := k8smeta.NewKubernetesClient()
	if err != nil {
		log.Fatalf("new kubernetes client: %v", err)
	}

	resolver := k8smeta.NewResolver(clientset, nodeName)
	if err := resolver.Start(ctx); err != nil {
		log.Fatalf("start resolver: %v", err)
	}

	agg := aggregator.NewTCPBaselineAggregator()

	c := collector.NewTCPConnectCollector(
		func(ev model.TCPConnectEvent) {
			if pod, ok := resolver.ResolveNetns(ev.NetnsIno); ok {
				log.Printf(
					"[resolved] netns=%d -> %s/%s workload=%s/%s",
					ev.NetnsIno,
					pod.Namespace,
					pod.PodName,
					pod.WorkloadKind,
					pod.WorkloadName,
				)
			} else {
				log.Printf("[resolved] netns=%d -> <unresolved>", ev.NetnsIno)
			}

			agg.Add(ev)
			collector.ExampleLogEvent(ev)
		},
		func(err error) {
			log.Printf("collector error: %v", err)
		},
	)

	if err := c.Start(ctx); err != nil {
		log.Fatalf("start collector: %v", err)
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

	log.Println("final baseline candidates:")
	printBaselineCandidatesAuto(agg)

	log.Println("flowmancer tcp connect collector stopped")
}
