package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"

	"erpagent-backend/internal/config"
	"erpagent-backend/internal/handlers"
	"erpagent-backend/middleware"
)

// TestSalesApp_EndToEndFlow verifies complete sales order flow:
// Quotation -> Confirmation -> Delivery -> eTIMS Fiscalization -> Credit Note -> Document Printing -> Approval Validation Prompts
func TestSalesApp_EndToEndFlow(t *testing.T) {
	// 1. Mock AI Engine Service
	aiEngineServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&req)
		prompt, _ := req["prompt"].(string)

		w.Header().Set("Content-Type", "application/json")

		switch prompt {
		case "Show me all active customers":
			_, _ = w.Write([]byte(`{
				"intent_type": "GET_CUSTOMERS",
				"confidence": 0.95,
				"action_type": "READ",
				"summary": "Fetch customers list",
				"parameters": {"search": ""},
				"validation": {"requires_user_approval": false}
			}`))
		case "Draft a quotation for Azure Interior for 10 units of Ergonomic Office Chair":
			_, _ = w.Write([]byte(`{
				"intent_type": "CREATE_QUOTATION",
				"confidence": 0.95,
				"action_type": "WRITE",
				"summary": "Draft Quotation for Azure Interior",
				"parameters": {
					"customer_name": "Azure Interior",
					"product_name": "Ergonomic Office Chair",
					"quantity": 10.0,
					"dry_run": true
				},
				"validation": {
					"requires_user_approval": true,
					"risk_level": "LOW",
					"approval_prompt": "Would you like to confirm and generate this Quotation for Azure Interior?"
				}
			}`))
		case "Create a sale order for Azure Interior for 5 units of Customizable Desk":
			_, _ = w.Write([]byte(`{
				"intent_type": "CREATE_SALE_ORDER",
				"confidence": 0.95,
				"action_type": "WRITE",
				"summary": "Create Sales Order for Azure Interior",
				"parameters": {
					"customer_name": "Azure Interior",
					"product_name": "Customizable Desk",
					"quantity": 5.0,
					"dry_run": true,
					"lines": [{"product_name": "Customizable Desk", "quantity": 5.0}]
				},
				"validation": {
					"requires_user_approval": true,
					"risk_level": "MEDIUM",
					"approval_prompt": "Do you want to confirm and validate the creation of Sale Order for Azure Interior?"
				}
			}`))
		case "Confirm sale order SO001":
			_, _ = w.Write([]byte(`{
				"intent_type": "CONFIRM_SALE_ORDER",
				"confidence": 0.95,
				"action_type": "WRITE",
				"summary": "Confirm Sales Order SO001",
				"parameters": {
					"order_id": "SO001",
					"dry_run": true
				},
				"validation": {
					"requires_user_approval": true,
					"risk_level": "MEDIUM",
					"approval_prompt": "Confirming Sales Order SO001 will lock inventory stock. Do you need this confirmed or validated?"
				}
			}`))
		case "Process delivery for sale order SO001":
			_, _ = w.Write([]byte(`{
				"intent_type": "DELIVER_SALE_ORDER",
				"confidence": 0.95,
				"action_type": "WRITE",
				"summary": "Validate stock delivery for Sales Order SO001",
				"parameters": {
					"order_id": "SO001",
					"dry_run": true
				},
				"validation": {
					"requires_user_approval": true,
					"risk_level": "HIGH",
					"approval_prompt": "Are you sure you want to validate and process inventory delivery for Sales Order SO001?"
				}
			}`))
		case "Submit invoice INV/2026/0001 to eTIMS for fiscalization":
			_, _ = w.Write([]byte(`{
				"intent_type": "SUBMIT_ETIMS",
				"confidence": 0.95,
				"action_type": "WRITE",
				"summary": "Submit Invoice INV/2026/0001 to KRA eTIMS for fiscalization",
				"parameters": {
					"invoice_id": "INV/2026/0001",
					"dry_run": true
				},
				"validation": {
					"requires_user_approval": true,
					"risk_level": "HIGH",
					"approval_prompt": "Fiscalizing Invoice INV/2026/0001 will transmit official tax data to KRA eTIMS. Do you confirm fiscalization?"
				}
			}`))
		case "Issue a credit note for Invoice INV/2026/0001 for 200 due to damaged goods":
			_, _ = w.Write([]byte(`{
				"intent_type": "CREATE_CREDIT_NOTE",
				"confidence": 0.95,
				"action_type": "WRITE",
				"summary": "Issue Credit Note for Invoice INV/2026/0001",
				"parameters": {
					"invoice_id": "INV/2026/0001",
					"amount": 200.0,
					"reason": "damaged goods",
					"dry_run": true
				},
				"validation": {
					"requires_user_approval": true,
					"risk_level": "HIGH",
					"approval_prompt": "HIGH IMPACT: Creating Credit Note for Invoice INV/2026/0001 will reverse funds. Do you confirm and validate this credit note?"
				}
			}`))
		case "Print pdf quotation for SO001":
			_, _ = w.Write([]byte(`{
				"intent_type": "PRINT_DOCUMENT",
				"confidence": 0.95,
				"action_type": "READ",
				"summary": "Generate printable PDF document for quotation SO001",
				"parameters": {
					"order_id": "SO001",
					"document_type": "quotation",
					"format": "pdf"
				},
				"validation": {"requires_user_approval": false}
			}`))
		default:
			_, _ = w.Write([]byte(`{
				"intent_type": "UNKNOWN",
				"confidence": 0.5,
				"action_type": "READ",
				"summary": "Unknown prompt",
				"parameters": {},
				"validation": {"requires_user_approval": false}
			}`))
		}
	}))
	defer aiEngineServer.Close()

	// 2. Setup Gateway App with Gorilla Mux Router
	cfg := &config.Config{
		Port:        "3000",
		AIEngineURL: aiEngineServer.URL,
	}

	r := mux.NewRouter()
	api := r.PathPrefix("/api/v1").Subrouter()
	protected := api.PathPrefix("/").Subrouter()
	protected.Use(middleware.TenantAuth)
	protected.HandleFunc("/chat", handlers.HandleChat(cfg)).Methods(http.MethodPost)

	// STEP 1: QUOTATION CREATION WITH VALIDATION PROMPT
	t.Run("Quotation Flow - Draft & Confirm Prompt", func(t *testing.T) {
		payload := []byte(`{"prompt": "Draft a quotation for Azure Interior for 10 units of Ergonomic Office Chair"}`)
		req := httptest.NewRequest("POST", "/api/v1/chat", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d. Body: %s", rec.Code, rec.Body.String())
		}

		var chatRes handlers.ChatResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &chatRes)

		if !chatRes.RequiresConfirmation {
			t.Errorf("expected RequiresConfirmation true")
		}
		if chatRes.ConfirmationPrompt == "" {
			t.Errorf("expected non-empty ConfirmationPrompt asking user if they need action validated")
		}
	})

	// STEP 2: DELIVERY ORDER VALIDATION
	t.Run("Delivery Flow - Stock Dispatch Validation", func(t *testing.T) {
		payload := []byte(`{"prompt": "Process delivery for sale order SO001"}`)
		req := httptest.NewRequest("POST", "/api/v1/chat", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		var chatRes handlers.ChatResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &chatRes)

		if chatRes.ParsedIntent.IntentType != "DELIVER_SALE_ORDER" {
			t.Errorf("expected DELIVER_SALE_ORDER intent, got %s", chatRes.ParsedIntent.IntentType)
		}
		if !chatRes.RequiresConfirmation {
			t.Errorf("expected RequiresConfirmation true for delivery validation")
		}
	})

	// STEP 3: eTIMS FISCALIZATION TRANSMISSION
	t.Run("eTIMS Flow - Fiscalization Transmission Preview", func(t *testing.T) {
		payload := []byte(`{"prompt": "Submit invoice INV/2026/0001 to eTIMS for fiscalization"}`)
		req := httptest.NewRequest("POST", "/api/v1/chat", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		var chatRes handlers.ChatResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &chatRes)

		if chatRes.ETIMSResult == nil {
			t.Fatalf("expected ETIMSResult payload in response")
		}
		if chatRes.ETIMSResult.CUSerialNo == "" || chatRes.ETIMSResult.IRN == "" {
			t.Errorf("expected CU Serial & IRN in eTIMS result")
		}
	})

	// STEP 4: CREDIT NOTE ISSUANCE
	t.Run("Credit Note Flow - Refund Validation", func(t *testing.T) {
		payload := []byte(`{"prompt": "Issue a credit note for Invoice INV/2026/0001 for 200 due to damaged goods"}`)
		req := httptest.NewRequest("POST", "/api/v1/chat", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		var chatRes handlers.ChatResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &chatRes)

		if chatRes.ParsedIntent.IntentType != "CREATE_CREDIT_NOTE" {
			t.Errorf("expected CREATE_CREDIT_NOTE intent, got %s", chatRes.ParsedIntent.IntentType)
		}
		if !chatRes.RequiresConfirmation {
			t.Errorf("expected RequiresConfirmation true for Credit Note issuance")
		}
	})

	// STEP 5: DOCUMENT PRINTING PDF
	t.Run("Document Printing - PDF Generation", func(t *testing.T) {
		payload := []byte(`{"prompt": "Print pdf quotation for SO001"}`)
		req := httptest.NewRequest("POST", "/api/v1/chat", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		bodyBytes, _ := io.ReadAll(rec.Body)
		var chatRes handlers.ChatResponse
		_ = json.Unmarshal(bodyBytes, &chatRes)

		if chatRes.PrintResult == nil {
			t.Fatalf("expected PrintResult in response")
		}
		if chatRes.PrintResult.DownloadURL == "" {
			t.Errorf("expected non-empty DownloadURL for printed document")
		}
	})
}
