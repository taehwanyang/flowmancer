package anomaly

import (
	"time"

	"github.com/taehwanyang/flowmancer/internal/aggregator"
)

type VolumeAnomalyConfig struct {
	MinSampleWindows int
	MinCurrentCount  uint64
	AvgMultiplier    float64
	P95Multiplier    float64
}

var DefaultVolumeAnomalyConfig = VolumeAnomalyConfig{
	MinSampleWindows: 10,
	MinCurrentCount:  20,
	AvgMultiplier:    3.0,
	P95Multiplier:    1.5,
}

func DetectVolumeAnomaly(
	now time.Time,
	current aggregator.ClosedWindow,
	historical aggregator.WorkloadFlowAggregate,
	cfg VolumeAnomalyConfig,
) *Result {
	if len(historical.WindowCounts) < cfg.MinSampleWindows {
		return nil
	}
	if current.Count < cfg.MinCurrentCount {
		return nil
	}

	avg := baselineAverageWindowCount(historical.WindowCounts)
	p95 := percentileUint64(historical.WindowCounts, 95)

	var evs []Evidence
	score := 0

	if avg > 0 && float64(current.Count) >= float64(avg)*cfg.AvgMultiplier {
		evs = append(evs, Evidence{
			Code:   "VOLUME_ABOVE_AVG",
			Score:  20,
			Reason: "current window count is significantly above baseline average",
		})
		score += 20
	}

	if p95 > 0 && float64(current.Count) >= float64(p95)*cfg.P95Multiplier {
		evs = append(evs, Evidence{
			Code:   "VOLUME_ABOVE_P95",
			Score:  20,
			Reason: "current window count exceeds baseline p95 range",
		})
		score += 20
	}

	if len(evs) == 0 {
		return nil
	}

	return &Result{
		Key:        current.Key,
		Score:      score,
		Severity:   severityFromScore(score),
		Evidences:  evs,
		DetectedAt: now,
	}
}
