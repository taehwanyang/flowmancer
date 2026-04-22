package aggregator

import (
	"time"
)

type BaselineSnapshot struct {
	GeneratedAt time.Time
	Flows       map[WorkloadFlowKey]WorkloadFlowAggregate
}

func (s *BaselineSnapshot) Get(key WorkloadFlowKey) (WorkloadFlowAggregate, bool) {
	if s == nil {
		return WorkloadFlowAggregate{}, false
	}

	agg, ok := s.Flows[key]
	if !ok {
		return WorkloadFlowAggregate{}, false
	}
	return cloneWorkloadFlowAggregate(agg), true
}

func (s *BaselineSnapshot) Len() int {
	if s == nil {
		return 0
	}
	return len(s.Flows)
}
