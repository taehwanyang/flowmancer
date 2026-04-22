package anomaly

import (
	"time"

	"github.com/taehwanyang/flowmancer/internal/aggregator"
)

type RareDestinationConfig struct {
	MaxDaysSeen        int
	MaxTotalCount      uint64
	MinCurrentWinCount uint64
	MinSpikeMultiplier float64
}

var DefaultRareDestinationConfig = RareDestinationConfig{
	MaxDaysSeen:        2,
	MaxTotalCount:      20,
	MinCurrentWinCount: 10,
	MinSpikeMultiplier: 3.0,
}

func DetectRareDestination(
	now time.Time,
	current aggregator.ClosedWindow,
	historical aggregator.WorkloadFlowAggregate,
	cfg RareDestinationConfig,
) *Result {
	daysSeen := len(historical.DaysSeen)
	isRare := daysSeen <= cfg.MaxDaysSeen || historical.Count <= cfg.MaxTotalCount
	if !isRare {
		return nil
	}

	if current.Count < cfg.MinCurrentWinCount {
		return nil
	}

	avg := baselineAverageWindowCount(historical.WindowCounts)
	if avg == 0 {
		return nil
	}

	if float64(current.Count) < float64(avg)*cfg.MinSpikeMultiplier {
		return nil
	}

	score := 30
	return &Result{
		Key:      current.Key,
		Score:    score,
		Severity: severityFromScore(score),
		Evidences: []Evidence{
			{
				Code:   "RARE_DESTINATION_SPIKE",
				Score:  30,
				Reason: "rare destination activity increased above expected level",
			},
		},
		DetectedAt: now,
	}
}
