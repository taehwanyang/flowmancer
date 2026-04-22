package anomaly

import (
	"time"

	"github.com/taehwanyang/flowmancer/internal/aggregator"
)

type Detector struct {
	WorkloadBaselineAggregator *aggregator.WorkloadBaselineAggregator
}

func NewDetector(b *aggregator.WorkloadBaselineAggregator) *Detector {
	return &Detector{WorkloadBaselineAggregator: b}
}

func (d *Detector) Evaluate(now time.Time, current aggregator.ClosedWindow) []*Result {
	historical, ok := d.WorkloadBaselineAggregator.Get(current.Key)

	if !ok {
		if r := DetectNewDestination(now, current, d.WorkloadBaselineAggregator); r != nil {
			return []*Result{r}
		}
		return nil
	}

	var out []*Result

	if r := DetectRareDestination(now, current, historical, DefaultRareDestinationConfig); r != nil {
		out = append(out, r)
	}
	if r := DetectVolumeAnomaly(now, current, historical, DefaultVolumeAnomalyConfig); r != nil {
		out = append(out, r)
	}

	return out
}
