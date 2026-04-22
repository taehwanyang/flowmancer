package main

import (
	"context"
	"log"
	"time"

	"github.com/taehwanyang/flowmancer/internal/aggregator"
	"github.com/taehwanyang/flowmancer/internal/anomaly"
)

func runDetectorWorker(
	ctx context.Context,
	detectCh <-chan aggregator.ClosedWindow,
	snapshotHolder *aggregator.BaselineSnapshotHolder,
	detector *anomaly.Detector,
) {
	log.Printf("[Anomaly Detector Worker] Started")

	for {
		select {
		case <-ctx.Done():
			log.Printf("[Anomaly Detector Worker] Stopped")
			return
		case cw, ok := <-detectCh:
			if !ok {
				log.Printf("[Anomaly Detector Worker] Channel closed, Worker Closing")
				return
			}

			snapshot := snapshotHolder.Get()
			if snapshot == nil {
				continue
			}

			// log.Printf("[Anomaly Detector Worker] Received ClosedWindow and Snapshot, Evaluate Will Start")

			results := detector.Evaluate(cw.WindowEnd, snapshot, cw)
			for _, r := range results {
				logAnomalyResult(cw, r)
			}
		}
	}
}

func logAnomalyResult(cw aggregator.ClosedWindow, r *anomaly.Result) {
	log.Printf(
		"[ANOMALY] severity=%s score=%d subject=%s dst=%s:%d evidences=%+v window=%s~%s",
		r.Severity,
		r.Score,
		aggregator.SubjectStringFromKey(r.Key),
		r.Key.Dst,
		r.Key.DstPort,
		r.Evidences,
		cw.WindowStart.Format(time.RFC3339),
		cw.WindowEnd.Format(time.RFC3339),
	)
}
