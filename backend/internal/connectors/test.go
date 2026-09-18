package connectors

import (
	"context"
	"fmt"
	"strings"

	"erpagent-backend/internal/adapters/odoo"
	"erpagent-backend/internal/domain"
	"erpagent-backend/internal/factory"
)

// ResolveTenantConfig maps a chat request onto ERP credentials (locked Q9).
// An explicit erp_type always wins (env-backed factory path); otherwise the
// tenant's stored "default" connector applies; else env defaults.
func ResolveTenantConfig(ctx context.Context, store Store, tenantID, erpType string, env factory.TenantConfig) factory.TenantConfig {
	env.TenantID = tenantID
	if strings.TrimSpace(erpType) != "" {
		env.ERPType = erpType
		return env
	}
	if store == nil {
		return env
	}
	def, err := store.Get(ctx, tenantID, "default")
	if err != nil || def == nil {
		return env
	}
	return factory.TenantConfig{
		TenantID: tenantID,
		ERPType:  def.ERPType,
		BaseURL:  def.BaseURL,
		DBName:   def.DBName,
		Username: def.Username,
		Password: def.Password,
	}
}

// TestResult is the outcome of probing a connector without persisting it.
type TestResult struct {
	OK      bool   `json:"ok"`
	Version string `json:"version,omitempty"`
	Message string `json:"message"`
}

// TestConnection builds the adapter and pings it: version probe for Odoo,
// catalog read for mock. Never persists.
func TestConnection(ctx context.Context, c TenantConnector) TestResult {
	connector, err := factory.GetERPConnector(factory.TenantConfig{
		TenantID: c.TenantID,
		ERPType:  c.ERPType,
		BaseURL:  c.BaseURL,
		DBName:   c.DBName,
		Username: c.Username,
		Password: c.Password,
	})
	if err != nil {
		return TestResult{OK: false, Message: fmt.Sprintf("adapter init failed: %v", err)}
	}

	if oa, ok := connector.(*odoo.OdooAdapter); ok {
		v := oa.DetectVersion(ctx)
		if _, err := connector.GetVendors(ctx, domain.VendorFilter{Limit: 1}); err != nil {
			// Version probe succeeded but data read failed: report honestly.
			return TestResult{OK: false, Version: v.Full, Message: fmt.Sprintf("connected (Odoo %s) but catalog read failed: %v", v.Full, err)}
		}
		return TestResult{OK: true, Version: v.Full, Message: fmt.Sprintf("Odoo %s reachable; vendor catalog readable", v.Full)}
	}

	if _, err := connector.GetVendors(ctx, domain.VendorFilter{Limit: 1}); err != nil {
		return TestResult{OK: false, Message: fmt.Sprintf("connector ping failed: %v", err)}
	}
	return TestResult{OK: true, Message: fmt.Sprintf("%s connector readable", c.ERPType)}
}
