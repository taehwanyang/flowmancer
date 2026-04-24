package anomaly

import (
	"time"

	"github.com/taehwanyang/flowmancer/internal/aggregator"
)

type Detector struct {
	Enabled bool

	NewDestinationEnabled  bool
	RareDestinationEnabled bool
	VolumeAnomalyEnabled   bool

	RareConfig   RareDestinationConfig
	VolumeConfig VolumeAnomalyConfig
}

func NewDetector() *Detector {
	return &Detector{
		Enabled: true,

		NewDestinationEnabled:  true,
		RareDestinationEnabled: true,
		VolumeAnomalyEnabled:   true,

		RareConfig:   DefaultRareDestinationConfig,
		VolumeConfig: DefaultVolumeAnomalyConfig,
	}
}

func (d *Detector) Evaluate(
	now time.Time,
	snapshot *aggregator.BaselineSnapshot,
	current aggregator.ClosedWindow,
) []*Result {
	if !d.Enabled {
		return nil
	}

	if snapshot == nil {
		return nil
	}

	historical, ok := snapshot.Get(current.Key)
	if !ok {
		if d.NewDestinationEnabled {
			return []*Result{DetectNewDestination(now, current)}
		}
		return nil
	}

	var out []*Result

	if d.RareDestinationEnabled {
		if r := DetectRareDestination(now, current, historical, d.RareConfig); r != nil {
			out = append(out, r)
		}
	}

	if d.VolumeAnomalyEnabled {
		if r := DetectVolumeAnomaly(now, current, historical, d.VolumeConfig); r != nil {
			out = append(out, r)
		}
	}

	return out
}
