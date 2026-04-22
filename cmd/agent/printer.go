package main

import (
	"log"

	"github.com/taehwanyang/flowmancer/internal/aggregator"
)

func printSnapshotTopN(builder *aggregator.BaselineBuilder, n int) {
	snapshot := builder.SnapshotTopN(n)
	log.Printf("current top %d workload flow aggregates: %d entries", n, len(snapshot))

	for _, item := range snapshot {
		log.Printf(
			"[top] subject=%s dst=%s:%d family=%d count=%d success=%d fail=%d avg_dur=%s first=%s last=%s",
			item.SubjectString(),
			item.DestinationString(),
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

func printBaselineCandidatesAuto(builder *aggregator.BaselineBuilder) {
	candidates, minCount := builder.BaselineCandidatesAuto()
	log.Printf("workload baseline candidates (auto minCount=%d): %d entries", minCount, len(candidates))

	for _, item := range candidates {
		log.Printf(
			"[baseline] subject=%s dst=%s:%d family=%d count=%d success=%d fail=%d success_ratio=%.2f avg_dur=%s",
			item.SubjectString(),
			item.DestinationString(),
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
