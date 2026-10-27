package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"erpagent-backend/internal/config"
	"erpagent-backend/internal/connectors"
	"erpagent-backend/internal/domain"
)

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v (%s)", err, rec.Body.String())
	}
	return body
}

func TestGetInvoices_HonorsRelationshipFilters(t *testing.T) {
	cfg := &config.Config{}
	handler := GetInvoices(cfg)

	t.Run("customer name resolves and overdue is enforced", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/sales/invoices?erp_type=MOCK&customer_name=Azure%20Interior&overdue=true", nil)
		rec := httptest.NewRecorder()
		handler(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		body := decodeBody(t, rec)
		if body["count"].(float64) != 1 {
			t.Fatalf("expected one overdue invoice, got %v", body["count"])
		}
		rows := body["data"].([]any)
		row := rows[0].(map[string]any)
		if row["customer_id"] != "10" {
			t.Errorf("expected resolved customer 10, got %v", row["customer_id"])
		}
		if row["payment_state"] != "not_paid" {
			t.Errorf("expected payment_state, got %v", row["payment_state"])
		}
	})

	t.Run("unknown customer is an explicit miss, not an empty list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/sales/invoices?erp_type=MOCK&customer_name=Nobody%20Here", nil)
		rec := httptest.NewRecorder()
		handler(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
		}
		if body := decodeBody(t, rec); body["error"] != "ENTITY_NOT_FOUND" {
			t.Errorf("expected ENTITY_NOT_FOUND, got %v", body["error"])
		}
	})

	t.Run("payment state filter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/sales/invoices?erp_type=MOCK&payment_state=paid", nil)
		rec := httptest.NewRecorder()
		handler(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if body := decodeBody(t, rec); body["count"].(float64) != 0 {
			t.Errorf("expected no paid invoices, got %v", body["count"])
		}
	})
}

func TestCountInvoices_ExactOverdue(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sales/invoices/count?erp_type=MOCK&overdue=true", nil)
	rec := httptest.NewRecorder()
	CountInvoices(&config.Config{})(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if body["count"].(float64) != 1 {
		t.Errorf("expected 1 overdue invoice, got %v", body["count"])
	}
	if meta := body["metadata"].(map[string]any); meta["exact"] != true {
		t.Errorf("expected exact count, got %v", meta)
	}
}

func TestGetSaleOrders_HonorsCustomerAndLimit(t *testing.T) {
	cfg := &config.Config{}
	handler := GetSaleOrders(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sales/orders?erp_type=MOCK&customer_name=azure%20interior&limit=1", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if body["count"].(float64) != 1 {
		t.Fatalf("expected one row, got %v", body["count"])
	}
	row := body["data"].([]any)[0].(map[string]any)
	if row["customer_id"] != "10" {
		t.Errorf("expected customer 10, got %v", row["customer_id"])
	}
	if meta := body["metadata"].(map[string]any); meta["limit"].(float64) != 1 {
		t.Errorf("expected limit echo, got %v", meta)
	}
}

func TestVerificationRejectsWrongRows(t *testing.T) {
	if err := verifySaleOrders([]domain.SaleOrder{{OrderNo: "SO1", CustomerID: "10", State: "sale"}}, domain.SaleOrderFilter{CustomerID: "12"}); err == nil {
		t.Error("expected customer mismatch to fail verification")
	}
	if err := verifySaleOrders([]domain.SaleOrder{{OrderNo: "SO1", CustomerID: "10", State: "draft"}}, domain.SaleOrderFilter{CustomerID: "10", State: "sale"}); err == nil {
		t.Error("expected state mismatch to fail verification")
	}

	past := time.Now().Add(-48 * time.Hour)
	future := time.Now().Add(48 * time.Hour)
	if err := verifyInvoices([]domain.Invoice{{InvoiceNo: "INV1", CustomerID: "10", Status: "posted", PaymentState: "not_paid", AmountDue: 100, DueDate: past}}, domain.InvoiceFilter{CustomerID: "10", Overdue: true}, time.Now()); err != nil {
		t.Errorf("expected valid overdue invoice to pass: %v", err)
	}
	if err := verifyInvoices([]domain.Invoice{{InvoiceNo: "INV1", Status: "posted", PaymentState: "not_paid", AmountDue: 100, DueDate: future}}, domain.InvoiceFilter{Overdue: true}, time.Now()); err == nil {
		t.Error("expected not-yet-due invoice to fail overdue verification")
	}
	if err := verifyInvoices([]domain.Invoice{{InvoiceNo: "INV1", Status: "posted", PaymentState: "paid", AmountDue: 0, DueDate: past}}, domain.InvoiceFilter{Overdue: true}, time.Now()); err == nil {
		t.Error("expected paid invoice to fail overdue verification")
	}
}

func TestNormalizeName(t *testing.T) {
	cases := map[string]string{
		"  Azure   Interior ": "azure interior",
		"ABC-Ltd.":            "abc ltd",
		"O'Brien & Sons":      "o brien sons",
	}
	for in, want := range cases {
		if got := normalizeName(in); got != want {
			t.Errorf("normalizeName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGetConnector_ExplicitERPTypeOverridesStoredDefault(t *testing.T) {
	store := connectors.NewMemoryStore()
	if _, err := store.Upsert(context.Background(), connectors.TenantConnector{
		TenantID: "t1", Name: "default", ERPType: "ODOO",
		BaseURL: "http://127.0.0.1:1", DBName: "unreachable",
	}); err != nil {
		t.Fatalf("seed connector: %v", err)
	}

	previous := GlobalConnStore
	GlobalConnStore = store
	defer func() { GlobalConnStore = previous }()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sales/customers?erp_type=MOCK", nil)
	rec := httptest.NewRecorder()
	GetCustomers(&config.Config{})(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected explicit MOCK to win, got %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if body["count"].(float64) == 0 {
		t.Fatalf("expected mock customers, got %v", body)
	}
}
