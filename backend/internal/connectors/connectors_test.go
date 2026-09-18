package connectors

import (
	"context"
	"testing"

	"erpagent-backend/internal/factory"
)

func TestMemoryStore_CRUD(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()

	saved, err := s.Upsert(ctx, TenantConnector{TenantID: "t1", ERPType: "MOCK"})
	if err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}
	if saved.Name != "default" {
		t.Errorf("expected default name, got %q", saved.Name)
	}

	got, err := s.Get(ctx, "t1", "default")
	if err != nil || got == nil || got.ERPType != "MOCK" {
		t.Errorf("Get failed: %v %+v", err, got)
	}

	list, _ := s.List(ctx, "t1")
	if len(list) != 1 {
		t.Errorf("expected 1 connector, got %d", len(list))
	}

	red := got.Redacted()
	if _, hasSecret := red["password"]; hasSecret {
		t.Errorf("redacted payload must not contain password: %+v", red)
	}
	if has, _ := red["has_secret"].(bool); has {
		t.Errorf("expected has_secret=false for empty password")
	}

	if err := s.Delete(ctx, "t1", "default"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if gone, _ := s.Get(ctx, "t1", "default"); gone != nil {
		t.Errorf("expected deleted connector to be gone")
	}
}

func TestResolveTenantConfig_Precedence(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()
	env := factory.TenantConfig{TenantID: "env-tenant", ERPType: "", BaseURL: "env-url"}

	// No stored connector, no explicit type: env passthrough.
	got := ResolveTenantConfig(ctx, s, "t1", "", env)
	if got.BaseURL != "env-url" || got.TenantID != "t1" {
		t.Errorf("expected env passthrough, got %+v", got)
	}

	// Stored default applies when no explicit type.
	_, _ = s.Upsert(ctx, TenantConnector{TenantID: "t1", Name: "default", ERPType: "ODOO", BaseURL: "odoo-url", DBName: "db"})
	got = ResolveTenantConfig(ctx, s, "t1", "", env)
	if got.ERPType != "ODOO" || got.BaseURL != "odoo-url" {
		t.Errorf("expected stored default, got %+v", got)
	}

	// Explicit type always wins.
	got = ResolveTenantConfig(ctx, s, "t1", "MOCK", env)
	if got.ERPType != "MOCK" {
		t.Errorf("expected explicit MOCK, got %+v", got)
	}
}

func TestTestConnection_Mock(t *testing.T) {
	ctx := context.Background()
	res := TestConnection(ctx, TenantConnector{TenantID: "t1", ERPType: "MOCK"})
	if !res.OK {
		t.Errorf("expected mock ping to succeed: %+v", res)
	}
}
