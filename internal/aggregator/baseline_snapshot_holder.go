package aggregator

import "sync"

type BaselineSnapshotHolder struct {
	mu       sync.RWMutex
	snapshot *BaselineSnapshot
}

func NewBaselineSnapshotHolder() *BaselineSnapshotHolder {
	return &BaselineSnapshotHolder{}
}

func (h *BaselineSnapshotHolder) Get() *BaselineSnapshot {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.snapshot
}

func (h *BaselineSnapshotHolder) Replace(snapshot *BaselineSnapshot) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.snapshot = snapshot
}
