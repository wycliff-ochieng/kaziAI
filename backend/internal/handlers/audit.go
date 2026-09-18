package handlers

import (
	"net/http"
	"strconv"

	"erpagent-backend/internal/audit"
	"erpagent-backend/internal/config"
	"erpagent-backend/middleware"
)

// GetAuditEvents serves the hash-chained audit trail (newest first).
// Query: ?limit=N (default 100, max 500). Tenant from auth context.
func GetAuditEvents(store audit.Store, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, _ := r.Context().Value(middleware.TenantIDKey).(string)

		limit := 100
		if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 {
			limit = v
		}

		events, err := store.List(r.Context(), tenantID, limit)
		if err != nil {
			RespondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if events == nil {
			events = []audit.Event{}
		}

		RespondJSON(w, http.StatusOK, map[string]any{
			"status": "success",
			"count":  len(events),
			"data":   events,
		})
	}
}

// ExportAuditCSV downloads the audit trail for auditors (oldest first).
func ExportAuditCSV(store audit.Store, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, _ := r.Context().Value(middleware.TenantIDKey).(string)

		events, err := store.List(r.Context(), tenantID, 500)
		if err != nil {
			RespondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		out, err := audit.ExportCSV(events)
		if err != nil {
			RespondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", `attachment; filename="audit-trail.csv"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(out))
	}
}

// VerifyAuditChain replays the hash chain and reports integrity.
func VerifyAuditChain(store audit.Store, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, _ := r.Context().Value(middleware.TenantIDKey).(string)

		checked, ok, err := store.Verify(r.Context(), tenantID)
		if err != nil {
			RespondJSON(w, http.StatusOK, map[string]any{
				"status":  "tampered",
				"checked": checked,
				"ok":      false,
				"error":   err.Error(),
			})
			return
		}

		RespondJSON(w, http.StatusOK, map[string]any{
			"status":  "verified",
			"checked": checked,
			"ok":      ok,
		})
	}
}
