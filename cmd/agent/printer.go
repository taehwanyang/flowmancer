package main

import (
	"log"

	"github.com/taehwanyang/flowmancer/internal/aggregator"
)

func printSnapshotTopN(agg *aggregator.TCPBaselineAggregator, n int) {
	snapshot := agg.SnapshotTopN(n)
	log.Printf("current top %d flow aggregates: %d entries", n, len(snapshot))

	for _, item := range snapshot {
		log.Printf(
			"[top] netns=%d comm=%s dst=%s:%d family=%d count=%d success=%d fail=%d avg_dur=%s first=%s last=%s",
			item.Key.NetnsIno,
			item.Key.Comm,
			item.Key.DstIP,
			item.Key.DstPort,
			item.Key.Family,
			item.Count,
			item.SuccessCount,
			item.FailCount,
			item.AvgDuration(),
			item.FirstSeen.Format("15:04:05"),
			item.LastSeen.Format("15:04:05"),
		)
	}
}

func printBaselineCandidatesAuto(agg *aggregator.TCPBaselineAggregator) {
	candidates, minCount := agg.BaselineCandidatesAuto()
	log.Printf("baseline candidates (auto minCount=%d): %d entries", minCount, len(candidates))

	for _, item := range candidates {
		log.Printf(
			"[baseline] netns=%d comm=%s dst=%s:%d family=%d count=%d success=%d fail=%d success_ratio=%.2f avg_dur=%s",
			item.Key.NetnsIno,
			item.Key.Comm,
			item.Key.DstIP,
			item.Key.DstPort,
			item.Key.Family,
			item.Count,
			item.SuccessCount,
			item.FailCount,
			item.SuccessRatio(),
			item.AvgDuration(),
		)
	}
}
