package handlers

import (
	"encoding/json"
	"net/http"
)

// RespondJSON is a utility helper to write JSON HTTP responses.
func RespondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// RespondError is a utility helper to write JSON error responses.
func RespondError(w http.ResponseWriter, status int, message string) {
	RespondJSON(w, status, map[string]string{"error": message})
}

// HealthCheck godoc
// @Summary System Health
// @Description returns system health status
// @Tags System
// @Produce json
// @Success 200 {object} map[string]string
// @Router /health [get]
func HealthCheck(w http.ResponseWriter, r *http.Request) {
	RespondJSON(w, http.StatusOK, map[string]string{
		"status":  "up",
		"service": "ERPAgent Gateway (Gorilla Mux)",
	})
}
