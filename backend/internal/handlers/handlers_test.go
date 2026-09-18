package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"

	"erpagent-backend/internal/config"
	"erpagent-backend/middleware"
)

func testConfig() *config.Config {
	return &config.Config{Port: "3000"}
}

func withTenant(r *http.Request) *http.Request {
	// Exercise real TenantAuth middleware instead of faking context.
	next := middleware.TenantAuth(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// stash the mutated request for the outer handler
		*r = *req
	}))
	rec := httptest.NewRecorder()
	next.ServeHTTP(rec, r)
	return r
}

func TestHealthCheckReportsUp(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	HealthCheck(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if body["status"] != "up" {
		t.Errorf("expected status up, got %v", body)
	}
}

func TestGetCustomersServesSeededCatalog(t *testing.T) {
	cfg := testConfig()
	req := withTenant(httptest.NewRequest(http.MethodGet, "/api/v1/sales/customers", nil))
	rec := httptest.NewRecorder()
	GetCustomers(cfg)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Status string        `json:"status"`
		Count  int           `json:"count"`
		Data   []interface{} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if body.Status != "success" || body.Count != 3 {
		t.Errorf("expected success with 3 customers, got %+v", body)
	}
}

func TestGetProductsFiltersBySearch(t *testing.T) {
	cfg := testConfig()
	req := withTenant(httptest.NewRequest(http.MethodGet, "/api/v1/sales/products?search=desk", nil))
	rec := httptest.NewRecorder()
	GetProducts(cfg)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body struct {
		Count int           `json:"count"`
		Data  []interface{} `json:"data"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if body.Count != 1 {
		t.Errorf("expected 1 desk product, got %d", body.Count)
	}
}

func TestGetSaleOrderByIDNotFoundReturns404(t *testing.T) {
	cfg := testConfig()
	req := withTenant(httptest.NewRequest(http.MethodGet, "/api/v1/sales/orders/NOPE", nil))
	req = mux.SetURLVars(req, map[string]string{"id": "NOPE"})
	rec := httptest.NewRecorder()
	GetSaleOrderByID(cfg)(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d body %s", rec.Code, rec.Body.String())
	}
}

func TestCreateSaleOrderRejectsInvalidPayload(t *testing.T) {
	cfg := testConfig()
	req := withTenant(httptest.NewRequest(http.MethodPost, "/api/v1/sales/orders",
		bytes.NewBufferString("{invalid json")))
	rec := httptest.NewRecorder()
	CreateSaleOrder(cfg)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid payload, got %d", rec.Code)
	}
}

func TestCreateSaleOrderDryRunPreviewThroughPublicAPI(t *testing.T) {
	cfg := testConfig()
	payload := `{"customer_id":"10","lines":[{"product_id":"101","quantity":2}],"dry_run":true}`
	req := withTenant(httptest.NewRequest(http.MethodPost, "/api/v1/sales/orders",
		bytes.NewBufferString(payload)))
	rec := httptest.NewRecorder()
	CreateSaleOrder(cfg)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body %s", rec.Code, rec.Body.String())
	}
	var body map[string]interface{}
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if _, ok := body["dry_run"]; !ok {
		t.Errorf("expected dry_run preview in response, got %v", body)
	}
}

func TestHandleChatRejectsEmptyPrompt(t *testing.T) {
	cfg := &config.Config{AIEngineURL: "http://127.0.0.1:1"}
	req := withTenant(httptest.NewRequest(http.MethodPost, "/api/v1/chat",
		bytes.NewBufferString(`{"prompt":"   "}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	HandleChat(cfg)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty prompt, got %d", rec.Code)
	}
}

func TestHandleChatRejectsInvalidBody(t *testing.T) {
	cfg := &config.Config{AIEngineURL: "http://127.0.0.1:1"}
	req := withTenant(httptest.NewRequest(http.MethodPost, "/api/v1/chat",
		bytes.NewBufferString("{bad")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	HandleChat(cfg)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid body, got %d", rec.Code)
	}
}

func TestHandleChatServesCustomersThroughMockAIEngine(t *testing.T) {
	ai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"intent_type": "GET_CUSTOMERS",
			"confidence": 0.95,
			"action_type": "READ",
			"summary": "Fetch customers list",
			"parameters": {"search": ""},
			"validation": {"requires_user_approval": false}
		}`))
	}))
	defer ai.Close()

	cfg := &config.Config{AIEngineURL: ai.URL}
	req := withTenant(httptest.NewRequest(http.MethodPost, "/api/v1/chat",
		bytes.NewBufferString(`{"prompt":"Show me all active customers"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	HandleChat(cfg)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body %s", rec.Code, rec.Body.String())
	}
	var chat ChatResponse
	if err := json.NewDecoder(rec.Body).Decode(&chat); err != nil {
		t.Fatalf("decode chat response failed: %v", err)
	}
	if chat.Status != "success" {
		t.Errorf("expected success status, got %s", chat.Status)
	}
}
