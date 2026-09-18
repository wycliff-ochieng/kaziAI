package odoo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"erpagent-backend/internal/domain"
)

func TestOdooAdapter_GetCustomers(t *testing.T) {
	// Mock Odoo Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
		}

		params, ok := req.Params.(map[string]interface{})
		if !ok {
			http.Error(w, "invalid params", http.StatusBadRequest)
			return
		}

		method := params["method"].(string)

		if method == "authenticate" {
			resp.Result = json.RawMessage(`2`)
		} else if method == "execute_kw" {
			args := params["args"].([]interface{})
			model := args[3].(string)
			kwMethod := args[4].(string)

			if model == "res.partner" && kwMethod == "search_read" {
				resp.Result = json.RawMessage(`[
					{"id": 10, "name": "Azure Interior", "email": "azure@example.com", "phone": "+12345", "credit_limit": 1000.0, "credit": 250.0, "street": "Main St", "city": "Nairobi"}
				]`)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	adapter := NewOdooAdapter(server.URL, "demo_db", "admin", "admin")
	customers, err := adapter.GetCustomers(context.Background(), domain.CustomerFilter{})
	if err != nil {
		t.Fatalf("unexpected error fetching customers: %v", err)
	}

	if len(customers) != 1 {
		t.Fatalf("expected 1 customer, got %d", len(customers))
	}

	if customers[0].Name != "Azure Interior" {
		t.Errorf("expected customer name 'Azure Interior', got '%s'", customers[0].Name)
	}
}

func TestOdooAdapter_CreateSaleOrder_DryRun(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		params := req.Params.(map[string]interface{})
		args := params["args"].([]interface{})
		model := args[3].(string)

		resp := jsonRPCResponse{JSONRPC: "2.0", ID: req.ID}

		if model == "res.partner" {
			resp.Result = json.RawMessage(`[{"id": 10, "name": "Azure Interior", "email": "a@a.com"}]`)
		} else if model == "product.product" {
			resp.Result = json.RawMessage(`[{"id": 101, "name": "Desk", "list_price": 200.0}]`)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	adapter := NewOdooAdapter(server.URL, "demo_db", "2", "pass")
	res, err := adapter.CreateSaleOrder(context.Background(), domain.CreateSaleOrderRequest{
		CustomerID: "10",
		Lines: []domain.CreateSaleOrderLineRequest{
			{ProductID: "101", Quantity: 2},
		},
		DryRun: true,
	})

	if err != nil {
		t.Fatalf("unexpected error on CreateSaleOrder DryRun: %v", err)
	}

	if !res.Success {
		t.Errorf("expected success true, got false")
	}

	if res.DryRun == nil || !res.DryRun.IsDryRun {
		t.Errorf("expected DryRun payload in response")
	}
}
