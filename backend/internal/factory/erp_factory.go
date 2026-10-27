package factory

import (
	"fmt"
	"os"
	"strings"

	"erpagent-backend/internal/adapters/mock"
	"erpagent-backend/internal/adapters/odoo"
	"erpagent-backend/internal/domain"
)

type TenantConfig struct {
	TenantID string `json:"tenant_id"`
	ERPType  string `json:"erp_type"` // "ODOO", "MOCK", "SAP", "DYNAMICS"
	BaseURL  string `json:"base_url"`
	DBName   string `json:"db_name"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// Global default mock instance for stateful mock interaction during testing
var defaultMockAdapter = mock.NewMockERPAdapter()

// GetERPConnector instantiates the appropriate ERP Adapter based on tenant configuration.
func GetERPConnector(cfg TenantConfig) (domain.ERPConnector, error) {
	erpType := strings.ToUpper(strings.TrimSpace(cfg.ERPType))

	// Fallback to environment configuration if ERP type not specified
	if erpType == "" {
		erpType = strings.ToUpper(os.Getenv("DEFAULT_ERP_TYPE"))
	}

	switch erpType {
	case "ODOO":
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = os.Getenv("ODOO_URL")
		}
		dbName := cfg.DBName
		if dbName == "" {
			dbName = os.Getenv("ODOO_DB")
		}
		username := cfg.Username
		if username == "" {
			username = os.Getenv("ODOO_USER")
		}
		password := cfg.Password
		if password == "" {
			password = os.Getenv("ODOO_PASSWORD")
		}

		if baseURL == "" || dbName == "" {
			// Fallback to mock adapter if Odoo env vars aren't populated
			return defaultMockAdapter, nil
		}
		return odoo.NewOdooAdapter(baseURL, dbName, username, password), nil

	case "MOCK", "":
		return defaultMockAdapter, nil

	case "POSTGRES", "MYSQL", "SAP", "DYNAMICS":
		// These connector types are listed for UI/save compatibility but have
		// no Go-side implementation. Returning a mock here would silently fake
		// success against a real ERP, so fail explicitly instead.
		return nil, fmt.Errorf("ERP type %s is saved but not implemented by this server", erpType)

	default:
		return nil, fmt.Errorf("unsupported or unimplemented ERP type: %s", erpType)
	}
}
