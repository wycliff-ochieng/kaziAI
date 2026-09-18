package connectors

import (
	"context"
	"sync"
	"time"
)

// TenantConnector is a per-tenant ERP connection (locked Q9: list/add/test).
// Passwords persist in Postgres in v1; move to Vault before multi-customer prod.
type TenantConnector struct {
	TenantID  string    `json:"tenant_id"`
	Name      string    `json:"name"` // "default" = used for chat when no erp_type given
	ERPType   string    `json:"erp_type"`
	BaseURL   string    `json:"base_url,omitempty"`
	DBName    string    `json:"db_name,omitempty"`
	Username  string    `json:"username,omitempty"`
	Password  string    `json:"-"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Redacted returns the connector safe for API responses.
func (c TenantConnector) Redacted() map[string]any {
	return map[string]any{
		"tenant_id":  c.TenantID,
		"name":       c.Name,
		"erp_type":   c.ERPType,
		"base_url":   c.BaseURL,
		"db_name":    c.DBName,
		"username":   c.Username,
		"has_secret": c.Password != "",
		"updated_at": c.UpdatedAt,
	}
}

// Store persists tenant connectors. MemoryStore serves tests/dev;
// PostgresStore is production.
type Store interface {
	Upsert(ctx context.Context, c TenantConnector) (TenantConnector, error)
	List(ctx context.Context, tenantID string) ([]TenantConnector, error)
	Get(ctx context.Context, tenantID, name string) (*TenantConnector, error)
	Delete(ctx context.Context, tenantID, name string) error
}

// MemoryStore is an in-process connector Store.
type MemoryStore struct {
	mu   sync.RWMutex
	data map[string]TenantConnector
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{data: make(map[string]TenantConnector)}
}

func connKey(tenantID, name string) string {
	return tenantID + "\x00" + name
}

func (m *MemoryStore) Upsert(ctx context.Context, c TenantConnector) (TenantConnector, error) {
	if c.Name == "" {
		c.Name = "default"
	}
	c.UpdatedAt = time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[connKey(c.TenantID, c.Name)] = c
	return c, nil
}

func (m *MemoryStore) List(ctx context.Context, tenantID string) ([]TenantConnector, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []TenantConnector
	for k, c := range m.data {
		if len(k) >= len(tenantID) && c.TenantID == tenantID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (m *MemoryStore) Get(ctx context.Context, tenantID, name string) (*TenantConnector, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.data[connKey(tenantID, name)]
	if !ok {
		return nil, nil
	}
	return &c, nil
}

func (m *MemoryStore) Delete(ctx context.Context, tenantID, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, connKey(tenantID, name))
	return nil
}
