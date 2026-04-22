package anomaly

import (
	"math"
	"sort"
)

func baselineAverageWindowCount(values []uint64) uint64 {
	if len(values) == 0 {
		return 0
	}

	var sum uint64
	for _, v := range values {
		sum += v
	}
	return sum / uint64(len(values))
}

func percentileUint64(values []uint64, p int) uint64 {
	if len(values) == 0 {
		return 0
	}

	cp := make([]uint64, len(values))
	copy(cp, values)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })

	if p <= 0 {
		return cp[0]
	}
	if p >= 100 {
		return cp[len(cp)-1]
	}

	idx := int(math.Ceil(float64(p)/100.0*float64(len(cp)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(cp) {
		idx = len(cp) - 1
	}
	return cp[idx]
}
