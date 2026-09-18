package factory

import (
	"testing"
)

func TestGetERPConnector_Mock(t *testing.T) {
	connector, err := GetERPConnector(TenantConfig{ERPType: "MOCK"})
	if err != nil {
		t.Fatalf("unexpected error getting mock connector: %v", err)
	}
	if connector == nil {
		t.Fatalf("expected non-nil connector")
	}
}

func TestGetERPConnector_Odoo(t *testing.T) {
	connector, err := GetERPConnector(TenantConfig{
		ERPType: "ODOO",
		BaseURL: "http://localhost:8069",
		DBName:  "test_db",
	})
	if err != nil {
		t.Fatalf("unexpected error getting odoo connector: %v", err)
	}
	if connector == nil {
		t.Fatalf("expected non-nil connector")
	}
}
