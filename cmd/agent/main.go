package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/taehwanyang/flowmancer/internal/aggregator"
	"github.com/taehwanyang/flowmancer/internal/collector"
	"github.com/taehwanyang/flowmancer/internal/model"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	agg := aggregator.NewTCPBaselineAggregator()

	c := collector.NewTCPConnectCollector(
		func(ev model.TCPConnectEvent) {
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
