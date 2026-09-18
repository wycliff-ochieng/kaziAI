package realtime

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

// PendingAction is a held dry-run awaiting conversational approval.
// The parsed intent is stored as raw JSON so this package never imports handlers.
type PendingAction struct {
	TenantID       string          `json:"tenant_id"`
	ConversationID string          `json:"conversation_id"`
	Prompt         string          `json:"prompt"`
	Intent         json.RawMessage `json:"intent"`
	CreatedUnix    int64           `json:"created_unix"`
}

// HistoryMsg is one turn fragment for windowed conversation memory.
type HistoryMsg struct {
	Role      string `json:"role"` // "user" | "assistant"
	Text      string `json:"text"`
	Timestamp int64  `json:"timestamp"`
}

// Store holds pending approvals, windowed history, and activity fan-out.
// MemoryStore serves tests/local dev; RedisStore serves shared deployments.
type Store interface {
	SavePending(ctx context.Context, p PendingAction) error
	TakePending(ctx context.Context, tenantID, conversationID string) (*PendingAction, error)
	DropPending(ctx context.Context, tenantID, conversationID string) error
	AppendHistory(ctx context.Context, tenantID, conversationID, role, text string) error
	History(ctx context.Context, tenantID, conversationID string, n int) ([]HistoryMsg, error)
	PublishActivity(ctx context.Context, event ActivityEvent) error
	SubscribeActivity(ctx context.Context, tenantID string) (<-chan ActivityEvent, func(), error)
}

// MemoryStore is an in-process Store for tests and single-instance dev.
type MemoryStore struct {
	mu       sync.RWMutex
	pending  map[string]PendingAction
	history  map[string][]HistoryMsg
	subs     map[string]map[chan ActivityEvent]struct{}
	histSize int
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		pending:  make(map[string]PendingAction),
		history:  make(map[string][]HistoryMsg),
		subs:     make(map[string]map[chan ActivityEvent]struct{}),
		histSize: 20,
	}
}

func pendingKey(tenantID, conversationID string) string {
	return tenantID + "\x00" + conversationID
}

func (m *MemoryStore) SavePending(ctx context.Context, p PendingAction) error {
	if p.CreatedUnix == 0 {
		p.CreatedUnix = time.Now().Unix()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pending[pendingKey(p.TenantID, p.ConversationID)] = p
	return nil
}

func (m *MemoryStore) TakePending(ctx context.Context, tenantID, conversationID string) (*PendingAction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.pending[pendingKey(tenantID, conversationID)]
	if !ok {
		return nil, nil
	}
	delete(m.pending, pendingKey(tenantID, conversationID))
	return &p, nil
}

func (m *MemoryStore) DropPending(ctx context.Context, tenantID, conversationID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.pending, pendingKey(tenantID, conversationID))
	return nil
}

func (m *MemoryStore) AppendHistory(ctx context.Context, tenantID, conversationID, role, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := pendingKey(tenantID, conversationID)
	m.history[k] = append(m.history[k], HistoryMsg{Role: role, Text: text, Timestamp: time.Now().Unix()})
	if len(m.history[k]) > m.histSize {
		m.history[k] = m.history[k][len(m.history[k])-m.histSize:]
	}
	return nil
}

func (m *MemoryStore) History(ctx context.Context, tenantID, conversationID string, n int) ([]HistoryMsg, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	h := m.history[pendingKey(tenantID, conversationID)]
	if n > 0 && len(h) > n {
		h = h[len(h)-n:]
	}
	out := make([]HistoryMsg, len(h))
	copy(out, h)
	return out, nil
}

func (m *MemoryStore) PublishActivity(ctx context.Context, event ActivityEvent) error {
	if event.Timestamp == 0 {
		event.Timestamp = time.Now().Unix()
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for ch := range m.subs[event.TenantID] {
		select {
		case ch <- event:
		default:
		}
	}
	return nil
}

func (m *MemoryStore) SubscribeActivity(ctx context.Context, tenantID string) (<-chan ActivityEvent, func(), error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ch := make(chan ActivityEvent, 32)
	if m.subs[tenantID] == nil {
		m.subs[tenantID] = make(map[chan ActivityEvent]struct{})
	}
	m.subs[tenantID][ch] = struct{}{}
	unsub := func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		delete(m.subs[tenantID], ch)
		close(ch)
	}
	return ch, unsub, nil
}
