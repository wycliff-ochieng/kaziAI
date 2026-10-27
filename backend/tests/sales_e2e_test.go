package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"

	"erpagent-backend/internal/config"
	"erpagent-backend/internal/handlers"
	"erpagent-backend/middleware"
)

// TestChatHTTP_UsesAgentTurn verifies the HTTP gateway contract after the brain
// move: /chat calls /process, returns the agent's typed result, and never
// executes a write without an approval step.
func TestChatHTTP_UsesAgentTurn(t *testing.T) {
	var seenThread string

	aiEngineServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/process" {
			t.Errorf("expected /api/v1/process, got %s", r.URL.Path)
		}

		var req map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&req)
		prompt, _ := req["prompt"].(string)
		seenThread, _ = req["thread_id"].(string)

		w.Header().Set("Content-Type", "application/json")
		switch prompt {
		case "Show me all active customers":
			_, _ = w.Write([]byte(`{
				"status": "success",
				"response": "Found 1 customer.",
				"intent_type": "GET_CUSTOMERS",
				"action_type": "READ",
				"data": [{"id": "10", "name": "Azure Interior"}],
				"count": 1
			}`))
		case "Draft a quotation for Azure Interior for 10 units of Ergonomic Office Chair":
			_, _ = w.Write([]byte(`{
				"status": "success",
				"response": "Preview ready — nothing has been created yet.",
				"intent_type": "CREATE_QUOTATION",
				"action_type": "WRITE",
				"requires_confirmation": true,
				"confirmation_prompt": "Create this quotation?",
				"plan": {"steps": [
					{"intent_type": "CREATE_QUOTATION", "parameters": {
						"customer_name": "Azure Interior",
						"product_name": "Ergonomic Office Chair",
						"quantity": 10.0,
						"dry_run": false
					}}
				]},
				"dry_run": {
					"is_dry_run": true,
					"operation": "CREATE_QUOTATION",
					"impact_summary": "Draft quotation for Azure Interior.",
					"risk_level": "LOW",
					"requires_confirmation": true,
					"confirmation_prompt": "Create this quotation?"
				}
			}`))
		default:
			_, _ = w.Write([]byte(`{
				"status": "success",
				"response": "How can I help with your Odoo data?",
				"intent_type": "CONVERSATION",
				"action_type": "CONVERSATION"
			}`))
		}
	}))
	defer aiEngineServer.Close()

	cfg := &config.Config{Port: "3000", AIEngineURL: aiEngineServer.URL}

	router := mux.NewRouter()
	api := router.PathPrefix("/api/v1").Subrouter()
	protected := api.PathPrefix("/").Subrouter()
	protected.Use(middleware.TenantAuth)
	protected.HandleFunc("/chat", handlers.HandleChat(cfg)).Methods(http.MethodPost)

	t.Run("read returns typed data", func(t *testing.T) {
		payload := []byte(`{"prompt": "Show me all active customers", "conversation_id": "c1"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/chat", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "t1")

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var res handlers.ChatResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if res.ParsedIntent == nil || res.ParsedIntent.IntentType != "GET_CUSTOMERS" {
			t.Fatalf("unexpected parsed intent: %+v", res.ParsedIntent)
		}
		if res.RequiresConfirmation {
			t.Errorf("reads must not require confirmation")
		}
		if !strings.HasPrefix(seenThread, "t1:") {
			t.Errorf("expected tenant-scoped thread id, got %q", seenThread)
		}
	})

	t.Run("write returns a preview plan, not an execution", func(t *testing.T) {
		payload := []byte(`{"prompt": "Draft a quotation for Azure Interior for 10 units of Ergonomic Office Chair", "conversation_id": "c1"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/chat", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "t1")

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var res handlers.ChatResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if !res.RequiresConfirmation || res.DryRun == nil {
			t.Fatalf("expected dry-run preview, got %+v", res)
		}
		if res.ParsedIntent == nil || res.ParsedIntent.IntentType != "CREATE_QUOTATION" {
			t.Fatalf("unexpected parsed intent: %+v", res.ParsedIntent)
		}
		if len(res.ParsedIntent.Steps) != 0 {
			t.Errorf("expected no follow-on steps for a single quotation, got %+v", res.ParsedIntent.Steps)
		}
	})

	t.Run("conversation does not touch ERP", func(t *testing.T) {
		payload := []byte(`{"prompt": "hello", "conversation_id": "c1"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/chat", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "t1")

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		var res handlers.ChatResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &res)
		if res.ParsedIntent == nil || res.ParsedIntent.IntentType != "CONVERSATION" {
			t.Errorf("expected CONVERSATION, got %+v", res.ParsedIntent)
		}
	})
}
