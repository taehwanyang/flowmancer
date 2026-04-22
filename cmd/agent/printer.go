package main

import (
	"context"
	"log"
	"time"

	"github.com/taehwanyang/flowmancer/internal/aggregator"
)

func scheduleBaselineDump(
	ctx context.Context,
	builder *aggregator.BaselineBuilder,
	delay time.Duration,
) {
	go func() {
		timer := time.NewTimer(delay)
		defer timer.Stop()

		select {
		case <-timer.C:
			snapshot := builder.Snapshot()

			log.Printf("===== BASELINE SNAPSHOT (after %s) =====", delay)

			for _, agg := range snapshot {
				key := agg.Key

				log.Printf(
					"[baseline] subject=%s dst=%s:%d family=%d count=%d success=%d fail=%d success_ratio=%.2f avg_dur=%s",
					aggregator.SubjectStringFromKey(key),
					key.Dst,
					key.DstPort,
					key.Family,
					agg.Count,
					agg.SuccessCount,
					agg.FailCount,
					agg.SuccessRatio(),
					agg.AvgDuration(),
				)
			}

			log.Printf("===== END BASELINE SNAPSHOT =====")

		case <-ctx.Done():
			return
		}
	}()
}
