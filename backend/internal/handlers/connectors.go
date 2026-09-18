package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gorilla/mux"

	"erpagent-backend/internal/config"
	"erpagent-backend/internal/connectors"
	"erpagent-backend/middleware"
)

var validERPTypes = map[string]bool{"MOCK": true, "ODOO": true, "SAP": true, "DYNAMICS": true}

// connectorRequest decodes secrets from the request body. TenantConnector
// itself carries `json:"-"` on Password so secrets can never serialize out.
type connectorRequest struct {
	Name     string `json:"name"`
	ERPType  string `json:"erp_type"`
	BaseURL  string `json:"base_url"`
	DBName   string `json:"db_name"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func (r connectorRequest) toConnector(tenantID string) connectors.TenantConnector {
	return connectors.TenantConnector{
		TenantID: tenantID,
		Name:     r.Name,
		ERPType:  strings.ToUpper(strings.TrimSpace(r.ERPType)),
		BaseURL:  r.BaseURL,
		DBName:   r.DBName,
		Username: r.Username,
		Password: r.Password,
	}
}

// ListConnectors returns the tenant's connectors with secrets redacted.
func ListConnectors(store connectors.Store, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, _ := r.Context().Value(middleware.TenantIDKey).(string)

		list, err := store.List(r.Context(), tenantID)
		if err != nil {
			RespondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		redacted := make([]map[string]any, 0, len(list))
		for _, c := range list {
			redacted = append(redacted, c.Redacted())
		}
		RespondJSON(w, http.StatusOK, map[string]any{
			"status": "success",
			"count":  len(redacted),
			"data":   redacted,
		})
	}
}

// UpsertConnector stores (or replaces) a tenant connector. Passwords persist
// in Postgres in v1; move to Vault before multi-customer prod.
func UpsertConnector(store connectors.Store, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, _ := r.Context().Value(middleware.TenantIDKey).(string)

		var req connectorRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			RespondError(w, http.StatusBadRequest, "Invalid connector payload")
			return
		}
		conn := req.toConnector(tenantID)
		if !validERPTypes[conn.ERPType] {
			RespondError(w, http.StatusBadRequest, "erp_type must be one of MOCK, ODOO, SAP, DYNAMICS")
			return
		}

		saved, err := store.Upsert(r.Context(), conn)
		if err != nil {
			RespondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		RespondJSON(w, http.StatusOK, map[string]any{"status": "success", "data": saved.Redacted()})
	}
}

// DeleteConnector removes a tenant connector by name.
func DeleteConnector(store connectors.Store, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, _ := r.Context().Value(middleware.TenantIDKey).(string)
		name := mux.Vars(r)["name"]

		if err := store.Delete(r.Context(), tenantID, name); err != nil {
			RespondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		RespondJSON(w, http.StatusOK, map[string]any{"status": "success"})
	}
}

// TestConnectorAdhoc probes credentials without persisting them.
func TestConnectorAdhoc(store connectors.Store, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, _ := r.Context().Value(middleware.TenantIDKey).(string)

		var req connectorRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			RespondError(w, http.StatusBadRequest, "Invalid connector payload")
			return
		}

		RespondJSON(w, http.StatusOK, connectors.TestConnection(r.Context(), req.toConnector(tenantID)))
	}
}

// TestConnectorStored probes a persisted connector by name.
func TestConnectorStored(store connectors.Store, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, _ := r.Context().Value(middleware.TenantIDKey).(string)
		name := mux.Vars(r)["name"]

		stored, err := store.Get(r.Context(), tenantID, name)
		if err != nil {
			RespondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if stored == nil {
			RespondError(w, http.StatusNotFound, "connector not found")
			return
		}

		RespondJSON(w, http.StatusOK, connectors.TestConnection(r.Context(), *stored))
	}
}
