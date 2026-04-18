package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/taehwanyang/flowmancer/internal/collector"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	c := collector.NewTCPConnectCollector(
		collector.ExampleLogEvent,
		func(err error) {
			log.Printf("collector error: %v", err)
		},
	)

	if err := c.Start(ctx); err != nil {
		log.Fatalf("start collector: %v", err)
	}

	log.Println("flowmancer tcp connect collector started")
	<-ctx.Done()
	log.Println("flowmancer tcp connect collector stopped")
}
