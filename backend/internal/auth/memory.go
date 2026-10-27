package auth

import (
	"context"
	"sync"
	"time"
)

type memorySession struct {
	userID    string
	expiresAt time.Time
}

// MemoryStore is a process-local auth store for local development and tests.
type MemoryStore struct {
	mu       sync.RWMutex
	users    map[string]*Account // by user id
	byEmail  map[string]string   // normalized email -> user id
	sessions map[string]memorySession
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		users:    make(map[string]*Account),
		byEmail:  make(map[string]string),
		sessions: make(map[string]memorySession),
	}
}

func (m *MemoryStore) CreateUser(_ context.Context, user User, passwordHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	email := NormalizeEmail(user.Email)
	if _, exists := m.byEmail[email]; exists {
		return ErrEmailTaken
	}
	now := time.Now().UTC()
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}
	user.UpdatedAt = now
	user.Email = email
	m.users[user.ID] = &Account{User: user, PasswordHash: passwordHash}
	m.byEmail[email] = user.ID
	return nil
}

func (m *MemoryStore) GetAccountByEmail(_ context.Context, email string) (*Account, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	id, ok := m.byEmail[NormalizeEmail(email)]
	if !ok {
		return nil, ErrNotFound
	}
	acct, ok := m.users[id]
	if !ok {
		return nil, ErrNotFound
	}
	copy := *acct
	return &copy, nil
}

func (m *MemoryStore) GetAccountByID(_ context.Context, id string) (*Account, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	acct, ok := m.users[id]
	if !ok {
		return nil, ErrNotFound
	}
	copy := *acct
	return &copy, nil
}

func (m *MemoryStore) GetUserByID(_ context.Context, id string) (*User, error) {
	acct, err := m.GetAccountByID(context.Background(), id)
	if err != nil {
		return nil, err
	}
	user := acct.User
	return &user, nil
}

func (m *MemoryStore) UpdateProfile(_ context.Context, id, name, email string) (*User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	acct, ok := m.users[id]
	if !ok {
		return nil, ErrNotFound
	}
	email = NormalizeEmail(email)
	if owner, exists := m.byEmail[email]; exists && owner != id {
		return nil, ErrEmailTaken
	}
	delete(m.byEmail, acct.Email)
	acct.Name = name
	acct.Email = email
	acct.UpdatedAt = time.Now().UTC()
	m.byEmail[email] = id
	user := acct.User
	return &user, nil
}

func (m *MemoryStore) UpdatePassword(_ context.Context, id, passwordHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	acct, ok := m.users[id]
	if !ok {
		return ErrNotFound
	}
	acct.PasswordHash = passwordHash
	acct.UpdatedAt = time.Now().UTC()
	return nil
}

func (m *MemoryStore) CreateSession(_ context.Context, userID string, ttl time.Duration) (string, error) {
	if _, ok := m.users[userID]; !ok {
		return "", ErrNotFound
	}
	token, tokenHash, err := NewSessionToken()
	if err != nil {
		return "", err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[tokenHash] = memorySession{userID: userID, expiresAt: time.Now().Add(ttl)}
	return token, nil
}

func (m *MemoryStore) GetUserBySession(_ context.Context, token string) (*User, error) {
	m.mu.RLock()
	session, ok := m.sessions[hashToken(token)]
	m.mu.RUnlock()
	if !ok || time.Now().After(session.expiresAt) {
		if ok {
			_ = m.DeleteSession(context.Background(), token)
		}
		return nil, ErrNotFound
	}
	return m.GetUserByID(context.Background(), session.userID)
}

func (m *MemoryStore) DeleteSession(_ context.Context, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, hashToken(token))
	return nil
}

func (m *MemoryStore) DeleteUserSessions(_ context.Context, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for token, session := range m.sessions {
		if session.userID == userID {
			delete(m.sessions, token)
		}
	}
	return nil
}
