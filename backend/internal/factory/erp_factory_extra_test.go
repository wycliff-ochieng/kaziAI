package factory

import (
	"testing"
)

func TestGetERPConnectorRejectsUnsupportedType(t *testing.T) {
	if _, err := GetERPConnector(TenantConfig{ERPType: "SAP"}); err == nil {
		t.Errorf("expected error for unsupported ERP type")
	}
}

func TestGetERPConnectorFallsBackToMockWithoutOdooEnv(t *testing.T) {
	connector, err := GetERPConnector(TenantConfig{ERPType: "ODOO"})
	if err != nil {
		t.Fatalf("expected mock fallback, got error: %v", err)
	}
	if connector == nil {
		t.Fatalf("expected non-nil fallback connector")
	}
}

func TestGetERPConnectorEmptyTypeReturnsSharedMock(t *testing.T) {
	first, err := GetERPConnector(TenantConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	second, err := GetERPConnector(TenantConfig{ERPType: "mock"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if first != second {
		t.Errorf("expected shared mock instance for empty/mock types")
	}
}
