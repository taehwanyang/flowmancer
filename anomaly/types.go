package anomaly

import (
	"time"

	"github.com/taehwanyang/flowmancer/internal/aggregator"
)

type Evidence struct {
	Code   string
	Score  int
	Reason string
}

type Result struct {
	Key        aggregator.WorkloadFlowKey
	Score      int
	Severity   string
	Evidences  []Evidence
	DetectedAt time.Time
}

func severityFromScore(score int) string {
	switch {
	case score >= 60:
		return "high"
	case score >= 30:
		return "medium"
	default:
		return "info"
	}
}
