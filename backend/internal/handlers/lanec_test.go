package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"

	"erpagent-backend/internal/audit"
	"erpagent-backend/internal/config"
	"erpagent-backend/internal/connectors"
)

func testRouter(auditStore audit.Store, connStore connectors.Store, cfg *config.Config) *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/audit", GetAuditEvents(auditStore, cfg)).Methods(http.MethodGet)
	r.HandleFunc("/audit/export", ExportAuditCSV(auditStore, cfg)).Methods(http.MethodGet)
	r.HandleFunc("/audit/verify", VerifyAuditChain(auditStore, cfg)).Methods(http.MethodGet)
	r.HandleFunc("/connectors", ListConnectors(connStore, cfg)).Methods(http.MethodGet)
	r.HandleFunc("/connectors", UpsertConnector(connStore, cfg)).Methods(http.MethodPost)
	r.HandleFunc("/connectors/test", TestConnectorAdhoc(connStore, cfg)).Methods(http.MethodPost)
	r.HandleFunc("/connectors/{name}", DeleteConnector(connStore, cfg)).Methods(http.MethodDelete)
	r.HandleFunc("/connectors/{name}/test", TestConnectorStored(connStore, cfg)).Methods(http.MethodPost)
	return r
}

func doJSON(t *testing.T, r *mux.Router, method, path string, body any) (int, map[string]any) {
	t.Helper()
	var buf *bytes.Buffer
	if body != nil {
		raw, _ := json.Marshal(body)
		buf = bytes.NewBuffer(raw)
	} else {
		buf = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, path, buf)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func TestLaneC_AuditAndConnectorsEndpoints(t *testing.T) {
	cfg := &config.Config{}
	auditStore := audit.NewMemoryStore()
	connStore := connectors.NewMemoryStore()
	r := testRouter(auditStore, connStore, cfg)

	// Audit starts empty.
	if code, out := doJSON(t, r, "GET", "/audit", nil); code != 200 || int(out["count"].(float64)) != 0 {
		t.Fatalf("expected empty audit, got %d %+v", code, out)
	}

	// Connectors start empty.
	if code, out := doJSON(t, r, "GET", "/connectors", nil); code != 200 || int(out["count"].(float64)) != 0 {
		t.Fatalf("expected empty connectors, got %d %+v", code, out)
	}

	// Ad-hoc MOCK test works without persisting.
	if code, out := doJSON(t, r, "POST", "/connectors/test", map[string]any{"erp_type": "MOCK"}); code != 200 || out["ok"] != true {
		t.Fatalf("expected ad-hoc test ok, got %d %+v", code, out)
	}

	// Upsert persists; secrets never leak.
	upsertBody := map[string]any{"name": "default", "erp_type": "MOCK", "password": "s3cret"}
	if code, out := doJSON(t, r, "POST", "/connectors", upsertBody); code != 200 {
		t.Fatalf("upsert failed: %d %+v", code, out)
	}
	if code, out := doJSON(t, r, "GET", "/connectors", nil); code != 200 || int(out["count"].(float64)) != 1 {
		t.Fatalf("expected 1 connector, got %d %+v", code, out)
	} else {
		data := out["data"].([]any)[0].(map[string]any)
		if _, leaked := data["password"]; leaked {
			t.Errorf("password leaked in list response")
		}
		if has, _ := data["has_secret"].(bool); !has {
			t.Errorf("expected has_secret=true")
		}
	}

	// Stored connector tests green.
	if code, out := doJSON(t, r, "POST", "/connectors/default/test", nil); code != 200 || out["ok"] != true {
		t.Fatalf("expected stored test ok, got %d %+v", code, out)
	}

	// Bad erp_type rejected.
	if code, _ := doJSON(t, r, "POST", "/connectors", map[string]any{"name": "x", "erp_type": "NOPE"}); code != http.StatusBadRequest {
		t.Errorf("expected 400 for bad erp_type, got %d", code)
	}

	// Audit row round-trips through the API with chain intact.
	_, _ = auditStore.Append(context.Background(), audit.Event{TenantID: "", Actor: "ws-user:c1", Channel: "ws-chat", Prompt: "show vendors", Intent: "GET_VENDORS", Approval: "none"})
	if code, out := doJSON(t, r, "GET", "/audit", nil); code != 200 || int(out["count"].(float64)) != 1 {
		t.Fatalf("expected 1 audit row, got %d %+v", code, out)
	}
	if code, out := doJSON(t, r, "GET", "/audit/verify", nil); code != 200 || out["ok"] != true {
		t.Fatalf("expected verified chain, got %d %+v", code, out)
	}

	req := httptest.NewRequest("GET", "/audit/export", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/csv") {
		t.Errorf("expected CSV export, got %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}

	// Delete cleans up.
	if code, _ := doJSON(t, r, "DELETE", "/connectors/default", nil); code != 200 {
		t.Errorf("delete failed: %d", code)
	}
}
