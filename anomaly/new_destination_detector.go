package anomaly

import (
	"time"

	"github.com/taehwanyang/flowmancer/internal/aggregator"
)

func DetectNewDestination(
	now time.Time,
	current aggregator.ClosedWindow,
) *Result {
	score := 40
	return &Result{
		Key:      current.Key,
		Score:    score,
		Severity: severityFromScore(score),
		Evidences: []Evidence{
			{
				Code:   "NEW_DESTINATION",
				Score:  40,
				Reason: "destination not found in baseline snapshot",
			},
		},
		DetectedAt: now,
	}
}
