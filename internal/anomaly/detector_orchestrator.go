package anomaly

import (
	"time"

	"github.com/taehwanyang/flowmancer/internal/aggregator"
)

type Detector struct {
	RareConfig   RareDestinationConfig
	VolumeConfig VolumeAnomalyConfig
}

func NewDetector() *Detector {
	return &Detector{
		RareConfig:   DefaultRareDestinationConfig,
		VolumeConfig: DefaultVolumeAnomalyConfig,
	}
}

func (d *Detector) Evaluate(
	now time.Time,
	snapshot *aggregator.BaselineSnapshot,
	current aggregator.ClosedWindow,
) []*Result {
	if snapshot == nil {
		return nil
	}

	historical, ok := snapshot.Get(current.Key)
	if !ok {
		return []*Result{DetectNewDestination(now, current)}
	}

	var out []*Result

	if r := DetectRareDestination(now, current, historical, d.RareConfig); r != nil {
		out = append(out, r)
	}

	if r := DetectVolumeAnomaly(now, current, historical, d.VolumeConfig); r != nil {
		out = append(out, r)
	}

	return out
}
