package chathistory

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
)

type memoryConversation struct {
	conversation Conversation
	userID       string
	messages     []Message
	nextID       int64
}

// MemoryStore is a process-local history store for local development and tests.
type MemoryStore struct {
	mu    sync.RWMutex
	convs map[string]*memoryConversation // tenant|conversation
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{convs: make(map[string]*memoryConversation)}
}

func memoryKey(tenantID, conversationID string) string {
	return tenantID + "|" + conversationID
}

func (m *MemoryStore) AppendMessage(_ context.Context, tenantID, userID, conversationID, role, content string) error {
	if conversationID == "" {
		return ErrNotFound
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	key := memoryKey(tenantID, conversationID)
	conv, ok := m.convs[key]
	if !ok {
		now := time.Now().UTC()
		conv = &memoryConversation{
			conversation: Conversation{ID: conversationID, Title: conversationTitle(role, content), CreatedAt: now, UpdatedAt: now},
			userID:       userID,
		}
		m.convs[key] = conv
	}
	if conv.userID != userID {
		return ErrNotFound
	}
	if role == "user" && (conv.conversation.Title == "" || conv.conversation.Title == "New conversation") {
		conv.conversation.Title = conversationTitle(role, content)
	}
	conv.nextID++
	conv.messages = append(conv.messages, Message{
		ID:        conv.nextID,
		Role:      role,
		Content:   content,
		CreatedAt: time.Now().UTC(),
	})
	conv.conversation.UpdatedAt = time.Now().UTC()
	conv.conversation.MessageCount = len(conv.messages)
	return nil
}

func (m *MemoryStore) ListConversations(_ context.Context, tenantID, userID string, limit int) ([]Conversation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Conversation, 0)
	for key, conv := range m.convs {
		if conv.userID != userID || !strings.HasPrefix(key, tenantID+"|") {
			continue
		}
		c := conv.conversation
		c.MessageCount = len(conv.messages)
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *MemoryStore) GetMessages(_ context.Context, tenantID, userID, conversationID string) ([]Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	conv, ok := m.convs[memoryKey(tenantID, conversationID)]
	if !ok || conv.userID != userID {
		return nil, ErrNotFound
	}
	out := make([]Message, len(conv.messages))
	copy(out, conv.messages)
	return out, nil
}

func (m *MemoryStore) DeleteConversation(_ context.Context, tenantID, userID, conversationID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := memoryKey(tenantID, conversationID)
	conv, ok := m.convs[key]
	if !ok || conv.userID != userID {
		return ErrNotFound
	}
	delete(m.convs, key)
	return nil
}
