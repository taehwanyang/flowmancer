package main

import (
	"log"

	"github.com/taehwanyang/flowmancer/internal/aggregator"
)

func printBaselineBuilder(builder *aggregator.BaselineBuilder) {
	candidates, _ := builder.BaselineCandidatesAuto()

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
