package audit

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MemoryStore is an in-process audit Store for tests and single-instance dev.
type MemoryStore struct {
	mu     sync.RWMutex
	events []Event
	seq    int64
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

func (m *MemoryStore) Append(ctx context.Context, e Event) (Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var prev string
	if len(m.events) > 0 {
		prev = m.events[len(m.events)-1].Hash
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	m.seq++
	e.ID = m.seq
	e.PrevHash = prev
	e.Hash = Chain(prev, e)
	m.events = append(m.events, e)
	return e, nil
}

func (m *MemoryStore) List(ctx context.Context, tenantID string, limit int) ([]Event, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var out []Event
	for i := len(m.events) - 1; i >= 0 && (limit <= 0 || len(out) < limit); i-- {
		if tenantID == "" || m.events[i].TenantID == tenantID {
			out = append(out, m.events[i])
		}
	}
	return out, nil
}

func (m *MemoryStore) Verify(ctx context.Context, tenantID string) (int64, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var prev string
	var checked int64
	for _, e := range m.events {
		if tenantID != "" && e.TenantID != tenantID {
			continue
		}
		if e.PrevHash != prev {
			return checked, false, fmt.Errorf("chain break at event %d", e.ID)
		}
		if Chain(prev, e) != e.Hash {
			return checked, false, fmt.Errorf("hash mismatch at event %d", e.ID)
		}
		prev = e.Hash
		checked++
	}
	return checked, true, nil
}
