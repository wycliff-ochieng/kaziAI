package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"erpagent-backend/internal/config"
	"erpagent-backend/internal/domain"
	"erpagent-backend/middleware"
)

type ChatRequest struct {
	Prompt         string `json:"prompt"`
	TenantID       string `json:"tenant_id,omitempty"`
	ConversationID string `json:"conversation_id,omitempty"`
	ERPType        string `json:"erp_type,omitempty"`
}

type OrderLinePayload struct {
	ProductName string  `json:"product_name"`
	ProductID   string  `json:"product_id"`
	Quantity    float64 `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
}

// QueryFilter is one natural-language constraint emitted by the AI engine.
type QueryFilter struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    any    `json:"value"`
}

type IntentParameters struct {
	CustomerName  string             `json:"customer_name"`
	CustomerID    string             `json:"customer_id"`
	VendorName    string             `json:"vendor_name"`
	VendorID      string             `json:"vendor_id"`
	BillID        string             `json:"bill_id"`
	ProductName   string             `json:"product_name"`
	ProductID     string             `json:"product_id"`
	OrderID       string             `json:"order_id"`
	InvoiceID     string             `json:"invoice_id"`
	QuotationID   string             `json:"quotation_id"`
	Reason        string             `json:"reason"`
	DocumentType  string             `json:"document_type"`
	Format        string             `json:"format"`
	Search        string             `json:"search"`
	State         string             `json:"state"`
	Quantity      float64            `json:"quantity"`
	UnitPrice     float64            `json:"unit_price"`
	Amount        float64            `json:"amount"`
	Lines         []OrderLinePayload `json:"lines"`
	Note          string             `json:"note"`
	DryRun        bool               `json:"dry_run"`
	ApprovalToken string             `json:"approval_token"`

	// Query IR (list/get/search/count semantics)
	Resource     string        `json:"resource"`
	Relationship string        `json:"relationship"`
	Filters      []QueryFilter `json:"filters"`
	Limit        int           `json:"limit"`
	Offset       int           `json:"offset"`
	Sort         string        `json:"sort"`
	// Domain is an AI-compiled, allowlisted Odoo domain (read-only here).
	Domain []any `json:"domain"`
}

// IntentStep is one link in a multi-intent chain (primary intent stays top-level).
type IntentStep struct {
	IntentType string           `json:"intent_type"`
	Summary    string           `json:"summary"`
	Parameters IntentParameters `json:"parameters"`
}

type ValidationRequirement struct {
	RequiresUserApproval bool   `json:"requires_user_approval"`
	RiskLevel            string `json:"risk_level"`
	ApprovalPrompt       string `json:"approval_prompt"`
	ImpactSummary        string `json:"impact_summary"`
}

type ParsedIntentResponse struct {
	IntentType string                `json:"intent_type"`
	Confidence float64               `json:"confidence"`
	ActionType string                `json:"action_type"`
	Summary    string                `json:"summary"`
	Parameters IntentParameters      `json:"parameters"`
	Validation ValidationRequirement `json:"validation"`
	// Steps carries ordered follow-on actions for compound requests.
	Steps []IntentStep `json:"steps,omitempty"`
}

type ProcessToolCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

type ProcessToolResult struct {
	Name    string `json:"name"`
	Content any    `json:"content"`
}

type ProcessPlan struct {
	Steps []IntentStep `json:"steps"`
}

type ProcessResponse struct {
	Status               string               `json:"status"`
	Prompt               string               `json:"prompt"`
	Response             string               `json:"response"`
	ThreadID             string               `json:"thread_id"`
	IntentType           string               `json:"intent_type"`
	ActionType           string               `json:"action_type"`
	Data                 any                  `json:"data"`
	Count                int                  `json:"count"`
	RequiresConfirmation bool                 `json:"requires_confirmation"`
	ConfirmationPrompt   string               `json:"confirmation_prompt"`
	Plan                 *ProcessPlan         `json:"plan"`
	DryRun               *domain.DryRunResult `json:"dry_run"`
	ToolCalls            []ProcessToolCall    `json:"tool_calls"`
	ToolResults          []ProcessToolResult  `json:"tool_results"`
}

type ChatResponse struct {
	Status               string                        `json:"status"` // "success", "approval_required", "dry_run_preview"
	Prompt               string                        `json:"prompt"`
	ParsedIntent         *ParsedIntentResponse         `json:"parsed_intent"`
	Message              string                        `json:"message"`
	RequiresConfirmation bool                          `json:"requires_confirmation"`
	ConfirmationPrompt   string                        `json:"confirmation_prompt,omitempty"`
	Data                 interface{}                   `json:"data,omitempty"`
	DryRun               *domain.DryRunResult          `json:"dry_run,omitempty"`
	PrintResult          *domain.PrintDocumentResult   `json:"print_result,omitempty"`
	ETIMSResult          *domain.ETIMSSubmissionResult `json:"etims_result,omitempty"`
}

func HandleChat(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			RespondError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		if strings.TrimSpace(req.Prompt) == "" {
			RespondError(w, http.StatusBadRequest, "Prompt cannot be empty")
			return
		}

		tenantID, _ := r.Context().Value(middleware.TenantIDKey).(string)
		if tenantID == "" {
			tenantID = req.TenantID
		}
		if tenantID == "" {
			tenantID = "default_tenant"
		}

		conversationID := strings.TrimSpace(req.ConversationID)
		if conversationID == "" {
			conversationID = "default"
		}
		threadID := tenantID + ":http:" + conversationID

		// 1. Call AI Engine for Natural Language Process (ReAct Loop)
		proc, err := callAIProcess(r.Context(), cfg.AIEngineURL, req.Prompt, tenantID, threadID, req.ERPType)
		if err != nil {
			RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"error":   "AI Engine process communication error",
				"details": err.Error(),
			})
			return
		}

		resp, _ := chatResponseFromProcess(proc, req.Prompt)
		RespondJSON(w, http.StatusOK, resp)
	}
}

func callAIProcess(ctx context.Context, aiURL, prompt, tenantID, threadID, erpType string) (*ProcessResponse, error) {
	reqPayload := map[string]interface{}{
		"prompt":    prompt,
		"tenant_id": tenantID,
		"thread_id": threadID,
		"erp_type":  erpType,
	}

	body, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, err
	}

	parseEndpoint := fmt.Sprintf("%s/api/v1/process", strings.TrimSuffix(aiURL, "/"))
	req, err := http.NewRequestWithContext(ctx, "POST", parseEndpoint, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Close = true

	client := &http.Client{
		Timeout: 180 * time.Second, // ReAct turns can include several LLM/tool round-trips
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http call to AI Engine /process failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AI Engine returned error code %d: %s", resp.StatusCode, string(respBytes))
	}

	var parsed ProcessResponse
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return nil, fmt.Errorf("failed to unmarshal AI process response: %w", err)
	}

	return &parsed, nil
}

// conversationalIntents are dialogue-only and must never touch the ERP.
var conversationalIntents = map[string]bool{
	"CONVERSATION": true,
	"CLARIFY":      true,
	"AFFIRM":       true,
	"REJECT":       true,
	"HELP":         true,
}

func isWriteIntent(intent string) bool {
	switch intent {
	case "CREATE_QUOTATION", "CREATE_SALE_ORDER", "CONFIRM_SALE_ORDER", "CANCEL_SALE_ORDER",
		"DELIVER_SALE_ORDER", "CREATE_INVOICE", "POST_INVOICE", "CREATE_CREDIT_NOTE", "SUBMIT_ETIMS",
		"CREATE_RFQ", "CONFIRM_PURCHASE_ORDER", "CANCEL_PURCHASE_ORDER", "RECEIVE_PURCHASE_ORDER",
		"CREATE_VENDOR_BILL", "POST_VENDOR_BILL", "APPROVE_ACTION":
		return true
	}
	return false
}

// executeIntent runs the primary intent, then threads any follow-on steps in a
// multi-intent chain ("create an SO and deliver it"). Steps only run once the
// primary action has genuinely executed; a held dry-run preview keeps them
// pending so approval stays a single decision.
func executeIntent(ctx context.Context, connector domain.ERPConnector, prompt string, parsed *ParsedIntentResponse) (*ChatResponse, error) {
	resp, err := executeIntentOnce(ctx, connector, prompt, parsed)
	if err != nil || resp == nil {
		return resp, err
	}

	// Every write case is written as a preview because that is the common path.
	// When the adapter actually executed (DryRun nil), report it as executed so
	// the UI does not re-issue an approval card for work that already happened.
	if resp.RequiresConfirmation && resp.DryRun == nil {
		resp.Status = "success"
		resp.RequiresConfirmation = false
		resp.ConfirmationPrompt = ""
	}

	if len(parsed.Steps) == 0 {
		return resp, nil
	}

	if resp.DryRun != nil {
		if planned := describeSteps(parsed.Steps); planned != "" {
			resp.Message = strings.TrimSpace(resp.Message + " Planned next: " + planned + ".")
		}
		return resp, nil
	}

	bag := map[string]string{}
	if id := extractRecordID(resp.Data); id != "" {
		bag["order_id"] = id
	}

	var outcomes []string
	for _, step := range parsed.Steps {
		params := step.Parameters
		if params.OrderID == "" {
			params.OrderID = bag["order_id"]
		}
		stepParsed := &ParsedIntentResponse{
			IntentType: step.IntentType,
			ActionType: "READ",
			Summary:    step.Summary,
			Parameters: params,
			Validation: parsed.Validation,
		}
		if isWriteIntent(step.IntentType) {
			stepParsed.ActionType = "WRITE"
			stepParsed.Parameters.DryRun = false
		}

		stepResp, stepErr := executeIntentOnce(ctx, connector, prompt, stepParsed)
		if stepErr != nil {
			return nil, fmt.Errorf("step %s failed: %w", step.IntentType, stepErr)
		}
		if stepResp != nil {
			if stepResp.Message != "" {
				outcomes = append(outcomes, stepResp.Message)
			}
			if id := extractRecordID(stepResp.Data); id != "" {
				bag["order_id"] = id
			}
		}
	}
	if len(outcomes) > 0 {
		resp.Message = strings.TrimSpace(resp.Message + " " + strings.Join(outcomes, " "))
	}
	return resp, nil
}

func describeSteps(steps []IntentStep) string {
	parts := make([]string, 0, len(steps))
	for _, s := range steps {
		parts = append(parts, strings.ToLower(strings.ReplaceAll(s.IntentType, "_", " ")))
	}
	return strings.Join(parts, " then ")
}

// extractRecordID recovers a created record's id from heterogeneous Data shapes.
func extractRecordID(data any) string {
	if data == nil {
		return ""
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}
	for _, key := range []string{"id", "order_id", "invoice_id"} {
		if v, ok := obj[key]; ok && v != nil {
			if s := fmt.Sprint(v); s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}

func executeIntentOnce(ctx context.Context, connector domain.ERPConnector, prompt string, parsed *ParsedIntentResponse) (*ChatResponse, error) {
	params := parsed.Parameters

	switch parsed.IntentType {

	case "CONVERSATION", "CLARIFY", "AFFIRM", "REJECT", "HELP":
		msg := strings.TrimSpace(parsed.Summary)
		if msg == "" {
			msg = "How can I help with your Odoo data?"
		}
		return &ChatResponse{
			Status:       "success",
			Prompt:       prompt,
			ParsedIntent: parsed,
			Message:      msg,
		}, nil

	case "GET_CUSTOMERS":
		filter := domain.CustomerFilter{Search: params.Search, Limit: params.Limit, Offset: params.Offset, Domain: params.Domain}
		customers, err := connector.GetCustomers(ctx, filter)
		if err != nil {
			return nil, err
		}
		msg := fmt.Sprintf("Found %d customer(s).", len(customers))
		if params.Search != "" {
			msg = fmt.Sprintf("Found %d customer(s) matching '%s'.", len(customers), params.Search)
		}
		return &ChatResponse{
			Status:       "success",
			Prompt:       prompt,
			ParsedIntent: parsed,
			Message:      msg,
			Data:         customers,
		}, nil

	case "GET_PRODUCTS":
		filter := domain.ProductFilter{Search: params.Search, Limit: params.Limit, Offset: params.Offset, Domain: params.Domain}
		products, err := connector.GetProducts(ctx, filter)
		if err != nil {
			return nil, err
		}
		msg := fmt.Sprintf("Found %d product(s) in catalog.", len(products))
		if params.Search != "" {
			msg = fmt.Sprintf("Found %d product(s) matching '%s'.", len(products), params.Search)
		}
		return &ChatResponse{
			Status:       "success",
			Prompt:       prompt,
			ParsedIntent: parsed,
			Message:      msg,
			Data:         products,
		}, nil

	case "GET_SALE_ORDERS":
		filter := domain.SaleOrderFilter{State: params.State, Search: params.Search, Limit: params.Limit, Offset: params.Offset, Domain: params.Domain}
		orders, err := connector.GetSaleOrders(ctx, filter)
		if err != nil {
			return nil, err
		}
		return &ChatResponse{
			Status:       "success",
			Prompt:       prompt,
			ParsedIntent: parsed,
			Message:      fmt.Sprintf("Retrieved %d sales order(s) / quotation(s).", len(orders)),
			Data:         orders,
		}, nil

	case "GET_SALE_ORDER_DETAILS":
		if params.OrderID == "" {
			return nil, fmt.Errorf("sales order ID not specified in request")
		}
		order, err := connector.GetSaleOrderByID(ctx, params.OrderID)
		if err != nil {
			return nil, err
		}
		return &ChatResponse{
			Status:       "success",
			Prompt:       prompt,
			ParsedIntent: parsed,
			Message:      fmt.Sprintf("Details for Sales Order %s (%s) retrieved successfully.", order.OrderNo, order.CustomerName),
			Data:         order,
		}, nil

	case "CREATE_QUOTATION":
		custID, lineReqs, err := resolveOrderParams(ctx, connector, params)
		if err != nil {
			return nil, err
		}

		createReq := domain.CreateSaleOrderRequest{
			CustomerID:  custID,
			Lines:       lineReqs,
			Note:        params.Note,
			IsQuotation: true,
			DryRun:      params.DryRun,
		}

		res, err := connector.CreateQuotation(ctx, createReq)
		if err != nil {
			return nil, err
		}

		promptQuery := parsed.Validation.ApprovalPrompt
		if promptQuery == "" {
			promptQuery = fmt.Sprintf("Do you want to validate and generate Quotation for customer %s?", params.CustomerName)
		}

		return &ChatResponse{
			Status:               "dry_run_preview",
			Prompt:               prompt,
			ParsedIntent:         parsed,
			Message:              res.Message,
			RequiresConfirmation: true,
			ConfirmationPrompt:   promptQuery,
			Data:                 res.Data,
			DryRun:               res.DryRun,
		}, nil

	case "SEND_QUOTATION":
		qID := params.QuotationID
		if qID == "" {
			qID = params.OrderID
		}
		if qID == "" {
			return nil, fmt.Errorf("quotation ID not specified")
		}

		res, err := connector.SendQuotation(ctx, qID)
		if err != nil {
			return nil, err
		}
		return &ChatResponse{
			Status:       "success",
			Prompt:       prompt,
			ParsedIntent: parsed,
			Message:      res.Message,
			Data:         res.Data,
		}, nil

	case "CONVERT_QUOTATION_TO_ORDER":
		qID := params.QuotationID
		if qID == "" {
			qID = params.OrderID
		}
		if qID == "" {
			return nil, fmt.Errorf("quotation ID required for conversion")
		}

		res, err := connector.ConvertQuotationToOrder(ctx, qID, params.DryRun)
		if err != nil {
			return nil, err
		}
		return &ChatResponse{
			Status:               "dry_run_preview",
			Prompt:               prompt,
			ParsedIntent:         parsed,
			Message:              res.Message,
			RequiresConfirmation: true,
			ConfirmationPrompt:   fmt.Sprintf("Do you want to confirm converting Quotation %s into a Confirmed Sale Order?", qID),
			Data:                 res.Data,
			DryRun:               res.DryRun,
		}, nil

	case "CREATE_SALE_ORDER":
		custID, lineReqs, err := resolveOrderParams(ctx, connector, params)
		if err != nil {
			return nil, err
		}

		createReq := domain.CreateSaleOrderRequest{
			CustomerID: custID,
			Lines:      lineReqs,
			Note:       params.Note,
			DryRun:     params.DryRun,
		}

		res, err := connector.CreateSaleOrder(ctx, createReq)
		if err != nil {
			return nil, err
		}

		confirmQuery := parsed.Validation.ApprovalPrompt
		if confirmQuery == "" {
			confirmQuery = fmt.Sprintf("Do you want to confirm and validate the creation of Sale Order for %s?", params.CustomerName)
		}

		return &ChatResponse{
			Status:               "dry_run_preview",
			Prompt:               prompt,
			ParsedIntent:         parsed,
			Message:              res.Message,
			RequiresConfirmation: true,
			ConfirmationPrompt:   confirmQuery,
			Data:                 res.Data,
			DryRun:               res.DryRun,
		}, nil

	case "CONFIRM_SALE_ORDER":
		if params.OrderID == "" {
			return nil, fmt.Errorf("sales order ID required for confirmation")
		}
		res, err := connector.ConfirmSaleOrder(ctx, params.OrderID, params.DryRun)
		if err != nil {
			return nil, err
		}

		return &ChatResponse{
			Status:               "dry_run_preview",
			Prompt:               prompt,
			ParsedIntent:         parsed,
			Message:              res.Message,
			RequiresConfirmation: true,
			ConfirmationPrompt:   fmt.Sprintf("Confirming Sales Order %s will lock inventory stock. Do you need this confirmed or validated?", params.OrderID),
			Data:                 res.Data,
			DryRun:               res.DryRun,
		}, nil

	case "DELIVER_SALE_ORDER":
		if params.OrderID == "" {
			return nil, fmt.Errorf("sales order ID required for delivery validation")
		}
		res, err := connector.DeliverSaleOrder(ctx, params.OrderID, params.DryRun)
		if err != nil {
			return nil, err
		}

		return &ChatResponse{
			Status:               "dry_run_preview",
			Prompt:               prompt,
			ParsedIntent:         parsed,
			Message:              res.Message,
			RequiresConfirmation: true,
			ConfirmationPrompt:   fmt.Sprintf("Are you sure you want to validate and process inventory delivery for Sales Order %s?", params.OrderID),
			Data:                 res.Data,
			DryRun:               res.DryRun,
		}, nil

	case "CANCEL_SALE_ORDER":
		if params.OrderID == "" {
			return nil, fmt.Errorf("sales order ID required for cancellation")
		}
		res, err := connector.CancelSaleOrder(ctx, params.OrderID, params.DryRun)
		if err != nil {
			return nil, err
		}
		return &ChatResponse{
			Status:               "dry_run_preview",
			Prompt:               prompt,
			ParsedIntent:         parsed,
			Message:              res.Message,
			RequiresConfirmation: true,
			ConfirmationPrompt:   fmt.Sprintf("CAUTION: Cancelling Sales Order %s will void order and release stock. Do you confirm cancellation?", params.OrderID),
			Data:                 res.Data,
			DryRun:               res.DryRun,
		}, nil

	case "CREATE_CREDIT_NOTE":
		invID := params.InvoiceID
		if invID == "" {
			invID = params.OrderID
		}
		if invID == "" {
			return nil, fmt.Errorf("invoice ID required for issuing credit note")
		}

		req := domain.CreditNoteRequest{
			InvoiceID: invID,
			Reason:    params.Reason,
			Amount:    params.Amount,
			DryRun:    params.DryRun,
		}

		res, err := connector.CreateCreditNote(ctx, req)
		if err != nil {
			return nil, err
		}
		return &ChatResponse{
			Status:               "dry_run_preview",
			Prompt:               prompt,
			ParsedIntent:         parsed,
			Message:              res.Message,
			RequiresConfirmation: true,
			ConfirmationPrompt:   fmt.Sprintf("HIGH IMPACT: Creating Credit Note against Invoice %s will refund funds. Do you confirm and validate this credit note?", invID),
			Data:                 res.Data,
			DryRun:               res.DryRun,
		}, nil

	case "SUBMIT_ETIMS":
		invID := params.InvoiceID
		if invID == "" {
			invID = params.OrderID
		}
		if invID == "" {
			return nil, fmt.Errorf("invoice ID required for eTIMS submission")
		}

		etimsRes, err := connector.SubmitETIMS(ctx, invID, params.DryRun)
		if err != nil {
			return nil, err
		}

		return &ChatResponse{
			Status:               "success",
			Prompt:               prompt,
			ParsedIntent:         parsed,
			Message:              etimsRes.Message,
			RequiresConfirmation: true,
			ConfirmationPrompt:   fmt.Sprintf("Fiscalizing Invoice %s with KRA eTIMS will transmit official tax payload. Confirm transmission?", invID),
			ETIMSResult:          etimsRes,
		}, nil

	case "PRINT_DOCUMENT":
		docID := params.OrderID
		if docID == "" {
			docID = params.InvoiceID
		}
		if docID == "" {
			docID = params.QuotationID
		}
		if docID == "" {
			docID = "SO001"
		}

		docType := params.DocumentType
		if docType == "" {
			docType = "sale_order"
		}

		printRes, err := connector.PrintDocument(ctx, domain.PrintDocumentRequest{
			DocumentType: docType,
			DocumentID:   docID,
			Format:       "pdf",
		})
		if err != nil {
			return nil, err
		}

		return &ChatResponse{
			Status:       "success",
			Prompt:       prompt,
			ParsedIntent: parsed,
			Message:      fmt.Sprintf("Printable PDF generated for %s document '%s'. Download URL ready.", docType, docID),
			PrintResult:  printRes,
		}, nil

	case "GET_INVOICES":
		invoices, err := connector.GetInvoices(ctx, domain.InvoiceFilter{
			CustomerID: params.CustomerID,
			Status:     params.State,
			Limit:      params.Limit,
			Offset:     params.Offset,
			Domain:     params.Domain,
		})
		if err != nil {
			return nil, err
		}
		return &ChatResponse{
			Status:       "success",
			Prompt:       prompt,
			ParsedIntent: parsed,
			Message:      fmt.Sprintf("Retrieved %d invoice(s).", len(invoices)),
			Data:         invoices,
		}, nil

	case "GET_OVERDUE_INVOICES":
		invoices, err := connector.GetOverdueInvoices(ctx, 0)
		if err != nil {
			return nil, err
		}
		return &ChatResponse{
			Status:       "success",
			Prompt:       prompt,
			ParsedIntent: parsed,
			Message:      fmt.Sprintf("Found %d overdue invoice(s).", len(invoices)),
			Data:         invoices,
		}, nil

	case "GET_VENDORS":
		vendors, err := connector.GetVendors(ctx, domain.VendorFilter{Search: params.Search, Limit: params.Limit, Offset: params.Offset, Domain: params.Domain})
		if err != nil {
			return nil, err
		}
		msg := fmt.Sprintf("Found %d vendor(s).", len(vendors))
		if params.Search != "" {
			msg = fmt.Sprintf("Found %d vendor(s) matching '%s'.", len(vendors), params.Search)
		}
		return &ChatResponse{
			Status:       "success",
			Prompt:       prompt,
			ParsedIntent: parsed,
			Message:      msg,
			Data:         vendors,
		}, nil

	case "GET_PURCHASE_ORDERS":
		filter := domain.PurchaseOrderFilter{State: params.State, Search: params.Search, Limit: params.Limit, Offset: params.Offset, Domain: params.Domain}
		orders, err := connector.GetPurchaseOrders(ctx, filter)
		if err != nil {
			return nil, err
		}
		return &ChatResponse{
			Status:       "success",
			Prompt:       prompt,
			ParsedIntent: parsed,
			Message:      fmt.Sprintf("Retrieved %d RFQ(s) / purchase order(s).", len(orders)),
			Data:         orders,
		}, nil

	case "GET_PURCHASE_ORDER_DETAILS":
		if params.OrderID == "" {
			return nil, fmt.Errorf("purchase order ID not specified in request")
		}
		order, err := connector.GetPurchaseOrderByID(ctx, params.OrderID)
		if err != nil {
			return nil, err
		}
		return &ChatResponse{
			Status:       "success",
			Prompt:       prompt,
			ParsedIntent: parsed,
			Message:      fmt.Sprintf("Details for PO %s (%s) retrieved successfully.", order.OrderNo, order.VendorName),
			Data:         order,
		}, nil

	case "CREATE_RFQ":
		vendorID, lineReqs, err := resolvePurchaseParams(ctx, connector, params)
		if err != nil {
			return nil, err
		}

		res, err := connector.CreateRFQ(ctx, domain.CreatePurchaseOrderRequest{
			VendorID: vendorID,
			Lines:    lineReqs,
			Note:     params.Note,
			DryRun:   params.DryRun,
		})
		if err != nil {
			return nil, err
		}

		confirmQuery := parsed.Validation.ApprovalPrompt
		if confirmQuery == "" {
			confirmQuery = fmt.Sprintf("Do you want to draft RFQ for %s?", params.VendorName)
		}

		return &ChatResponse{
			Status:               "dry_run_preview",
			Prompt:               prompt,
			ParsedIntent:         parsed,
			Message:              res.Message,
			RequiresConfirmation: true,
			ConfirmationPrompt:   confirmQuery,
			Data:                 res.Data,
			DryRun:               res.DryRun,
		}, nil

	case "CONFIRM_PURCHASE_ORDER":
		if params.OrderID == "" {
			return nil, fmt.Errorf("purchase order ID required for confirmation")
		}
		res, err := connector.ConfirmPurchaseOrder(ctx, params.OrderID, params.DryRun)
		if err != nil {
			return nil, err
		}
		return &ChatResponse{
			Status:               "dry_run_preview",
			Prompt:               prompt,
			ParsedIntent:         parsed,
			Message:              res.Message,
			RequiresConfirmation: true,
			ConfirmationPrompt:   fmt.Sprintf("Confirming PO %s commits spend. Do you want to proceed?", params.OrderID),
			Data:                 res.Data,
			DryRun:               res.DryRun,
		}, nil

	case "RECEIVE_PURCHASE_ORDER":
		if params.OrderID == "" {
			return nil, fmt.Errorf("purchase order ID required for receipt")
		}
		res, err := connector.ReceivePurchaseOrder(ctx, params.OrderID, params.DryRun)
		if err != nil {
			return nil, err
		}
		return &ChatResponse{
			Status:               "dry_run_preview",
			Prompt:               prompt,
			ParsedIntent:         parsed,
			Message:              res.Message,
			RequiresConfirmation: true,
			ConfirmationPrompt:   fmt.Sprintf("Receive ALL lines of PO %s into stock?", params.OrderID),
			Data:                 res.Data,
			DryRun:               res.DryRun,
		}, nil

	case "CREATE_VENDOR_BILL":
		poID := params.OrderID
		if poID == "" {
			return nil, fmt.Errorf("purchase order ID required for vendor bill creation")
		}
		res, err := connector.CreateVendorBill(ctx, poID, params.DryRun)
		if err != nil {
			return nil, err
		}
		return &ChatResponse{
			Status:               "dry_run_preview",
			Prompt:               prompt,
			ParsedIntent:         parsed,
			Message:              res.Message,
			RequiresConfirmation: true,
			ConfirmationPrompt:   fmt.Sprintf("Draft vendor bill from PO %s?", poID),
			Data:                 res.Data,
			DryRun:               res.DryRun,
		}, nil

	case "POST_VENDOR_BILL":
		billID := params.BillID
		if billID == "" {
			billID = params.OrderID
		}
		if billID == "" {
			return nil, fmt.Errorf("vendor bill ID required for posting")
		}
		res, err := connector.PostVendorBill(ctx, billID, params.DryRun)
		if err != nil {
			return nil, err
		}
		return &ChatResponse{
			Status:               "dry_run_preview",
			Prompt:               prompt,
			ParsedIntent:         parsed,
			Message:              res.Message,
			RequiresConfirmation: true,
			ConfirmationPrompt:   fmt.Sprintf("Post vendor bill %s? (Journal only, no payment.)", billID),
			Data:                 res.Data,
			DryRun:               res.DryRun,
		}, nil

	case "CANCEL_PURCHASE_ORDER":
		if params.OrderID == "" {
			return nil, fmt.Errorf("purchase order ID required for cancellation")
		}
		res, err := connector.CancelPurchaseOrder(ctx, params.OrderID, params.DryRun)
		if err != nil {
			return nil, err
		}
		return &ChatResponse{
			Status:               "dry_run_preview",
			Prompt:               prompt,
			ParsedIntent:         parsed,
			Message:              res.Message,
			RequiresConfirmation: true,
			ConfirmationPrompt:   fmt.Sprintf("Cancel draft RFQ %s?", params.OrderID),
			Data:                 res.Data,
			DryRun:               res.DryRun,
		}, nil

	case "COUNT_RECORDS":
		count, err := countRecords(ctx, connector, params)
		if err != nil {
			return nil, err
		}
		return &ChatResponse{
			Status:       "success",
			Prompt:       prompt,
			ParsedIntent: parsed,
			Message:      count.message,
			Data:         map[string]any{"resource": count.resource, "count": count.value, "capped": count.capped},
		}, nil

	default:
		return &ChatResponse{
			Status:       "unknown_intent",
			Prompt:       prompt,
			ParsedIntent: parsed,
			Message: fmt.Sprintf(
				"Could not map that request to an ERP action yet (understood as %s). "+
					"Try naming what you want — e.g. \"show first 10 customers\", \"show unpaid invoices\" or \"help\".",
				parsed.IntentType),
		}, nil
	}
}

// countScanLimit bounds an in-process count so we never pull an unbounded set.
const countScanLimit = 500

type recordCount struct {
	resource string
	value    int
	capped   bool
	message  string
}

// countRecords answers "how many" without conflating it with a list request.
// Odoo search_count is not exposed through the portable connector yet, so we
// scan a bounded page and say so when the cap is reached.
func countRecords(ctx context.Context, connector domain.ERPConnector, params IntentParameters) (recordCount, error) {
	resource := strings.ToLower(strings.TrimSpace(params.Resource))
	var n int

	switch resource {
	case "customers", "customer":
		rows, err := connector.GetCustomers(ctx, domain.CustomerFilter{Search: params.Search, Domain: params.Domain, Limit: countScanLimit})
		if err != nil {
			return recordCount{}, err
		}
		n = len(rows)
	case "vendors", "vendor", "suppliers":
		rows, err := connector.GetVendors(ctx, domain.VendorFilter{Search: params.Search, Domain: params.Domain, Limit: countScanLimit})
		if err != nil {
			return recordCount{}, err
		}
		n = len(rows)
	case "products", "product":
		rows, err := connector.GetProducts(ctx, domain.ProductFilter{Search: params.Search, Domain: params.Domain, Limit: countScanLimit})
		if err != nil {
			return recordCount{}, err
		}
		n = len(rows)
	case "orders", "sale_orders", "sales_orders":
		rows, err := connector.GetSaleOrders(ctx, domain.SaleOrderFilter{State: params.State, Search: params.Search, Domain: params.Domain, Limit: countScanLimit})
		if err != nil {
			return recordCount{}, err
		}
		n = len(rows)
	case "purchase_orders", "rfqs":
		rows, err := connector.GetPurchaseOrders(ctx, domain.PurchaseOrderFilter{State: params.State, Search: params.Search, Domain: params.Domain, Limit: countScanLimit})
		if err != nil {
			return recordCount{}, err
		}
		n = len(rows)
	case "invoices":
		rows, err := connector.GetInvoices(ctx, domain.InvoiceFilter{Status: params.State, Domain: params.Domain, Limit: countScanLimit})
		if err != nil {
			return recordCount{}, err
		}
		n = len(rows)
	default:
		return recordCount{}, fmt.Errorf("cannot count resource %q", params.Resource)
	}

	capped := n >= countScanLimit
	label := strings.ReplaceAll(resource, "_", " ")
	message := fmt.Sprintf("There are %d %s matching that.", n, label)
	if capped {
		message = fmt.Sprintf("There are at least %d %s matching that (count capped at %d).", n, label, countScanLimit)
	}
	return recordCount{resource: resource, value: n, capped: capped, message: message}, nil
}

// resolvePartnerID prefers an exact (case-insensitive) name and only accepts a
// lone fuzzy candidate. Several matches become a clarification error rather
// than a silent guess between duplicates.
func resolvePartnerID(query string, names []string, ids []string) (string, error) {
	exact := make([]int, 0, len(ids))
	for i, n := range names {
		if strings.EqualFold(strings.TrimSpace(n), strings.TrimSpace(query)) {
			exact = append(exact, i)
		}
	}
	switch {
	case len(exact) == 1:
		return ids[exact[0]], nil
	case len(exact) > 1:
		return "", fmt.Errorf("multiple records match %q (%s); please specify which one", query, strings.Join(names, ", "))
	case len(ids) == 1:
		return ids[0], nil
	default:
		return "", fmt.Errorf("multiple records match %q (%s); please specify which one", query, strings.Join(names, ", "))
	}
}

// resolveProductID matches on name or SKU, and refuses to pick between several.
func resolveProductID(query string, names []string, skus []string, ids []string) (string, error) {
	exact := make([]int, 0, len(ids))
	for i := range ids {
		if strings.EqualFold(strings.TrimSpace(names[i]), strings.TrimSpace(query)) ||
			strings.EqualFold(strings.TrimSpace(skus[i]), strings.TrimSpace(query)) {
			exact = append(exact, i)
		}
	}
	if len(exact) == 1 {
		return ids[exact[0]], nil
	}
	if len(exact) == 0 && len(ids) == 1 {
		return ids[0], nil
	}
	labels := make([]string, 0, len(ids))
	for i := range ids {
		label := names[i]
		if skus[i] != "" {
			label += " (" + skus[i] + ")"
		}
		labels = append(labels, label)
	}
	return "", fmt.Errorf("multiple products match %q (%s); please specify which one", query, strings.Join(labels, ", "))
}

func resolvePurchaseParams(ctx context.Context, connector domain.ERPConnector, params IntentParameters) (string, []domain.CreatePurchaseOrderLineRequest, error) {
	vendorID := params.VendorID
	if vendorID == "" && params.VendorName != "" {
		vendors, err := connector.GetVendors(ctx, domain.VendorFilter{Search: params.VendorName, Limit: 5})
		if err == nil && len(vendors) > 0 {
			names := make([]string, 0, len(vendors))
			ids := make([]string, 0, len(vendors))
			for _, v := range vendors {
				names = append(names, v.Name)
				ids = append(ids, v.ID)
			}
			picked, perr := resolvePartnerID(params.VendorName, names, ids)
			if perr != nil {
				return "", nil, perr
			}
			vendorID = picked
		}
	}
	if vendorID == "" {
		if params.VendorName != "" {
			return "", nil, fmt.Errorf("could not find vendor matching: %s", params.VendorName)
		}
		return "", nil, fmt.Errorf("vendor ID or Name is required for RFQ creation")
	}

	var lineReqs []domain.CreatePurchaseOrderLineRequest
	if len(params.Lines) > 0 {
		for _, line := range params.Lines {
			prodID := line.ProductID
			if prodID == "" && line.ProductName != "" {
				prods, err := connector.GetProducts(ctx, domain.ProductFilter{Search: line.ProductName, Limit: 5})
				if err == nil && len(prods) > 0 {
					names := make([]string, 0, len(prods))
					skus := make([]string, 0, len(prods))
					ids := make([]string, 0, len(prods))
					for _, p := range prods {
						names = append(names, p.Name)
						skus = append(skus, p.SKU)
						ids = append(ids, p.ID)
					}
					picked, perr := resolveProductID(line.ProductName, names, skus, ids)
					if perr != nil {
						return "", nil, perr
					}
					prodID = picked
				}
			}
			if prodID != "" {
				lineReqs = append(lineReqs, domain.CreatePurchaseOrderLineRequest{
					ProductID: prodID,
					Quantity:  line.Quantity,
					UnitPrice: line.UnitPrice,
				})
			}
		}
	}

	if len(lineReqs) == 0 {
		prodID := params.ProductID
		if prodID == "" && params.ProductName != "" {
			prods, err := connector.GetProducts(ctx, domain.ProductFilter{Search: params.ProductName, Limit: 5})
			if err == nil && len(prods) > 0 {
				names := make([]string, 0, len(prods))
				skus := make([]string, 0, len(prods))
				ids := make([]string, 0, len(prods))
				for _, p := range prods {
					names = append(names, p.Name)
					skus = append(skus, p.SKU)
					ids = append(ids, p.ID)
				}
				picked, perr := resolveProductID(params.ProductName, names, skus, ids)
				if perr != nil {
					return "", nil, perr
				}
				prodID = picked
			}
		}
		if prodID == "" {
			if params.ProductName != "" {
				return "", nil, fmt.Errorf("could not find product matching: %s", params.ProductName)
			}
			return "", nil, fmt.Errorf("product ID or Name is required")
		}
		qty := params.Quantity
		if qty <= 0 {
			qty = 1.0
		}
		lineReqs = append(lineReqs, domain.CreatePurchaseOrderLineRequest{
			ProductID: prodID,
			Quantity:  qty,
			UnitPrice: params.UnitPrice,
		})
	}

	return vendorID, lineReqs, nil
}

func resolveOrderParams(ctx context.Context, connector domain.ERPConnector, params IntentParameters) (string, []domain.CreateSaleOrderLineRequest, error) {
	custID := params.CustomerID
	if custID == "" && params.CustomerName != "" {
		custs, err := connector.GetCustomers(ctx, domain.CustomerFilter{Search: params.CustomerName, Limit: 5})
		if err == nil && len(custs) > 0 {
			names := make([]string, 0, len(custs))
			ids := make([]string, 0, len(custs))
			for _, c := range custs {
				names = append(names, c.Name)
				ids = append(ids, c.ID)
			}
			picked, perr := resolvePartnerID(params.CustomerName, names, ids)
			if perr != nil {
				return "", nil, perr
			}
			custID = picked
		}
	}
	if custID == "" {
		if params.CustomerName != "" {
			return "", nil, fmt.Errorf("could not find customer matching: %s", params.CustomerName)
		}
		return "", nil, fmt.Errorf("customer ID or Name is required for sales order creation")
	}

	var lineReqs []domain.CreateSaleOrderLineRequest
	if len(params.Lines) > 0 {
		for _, line := range params.Lines {
			prodID := line.ProductID
			if prodID == "" && line.ProductName != "" {
				prods, err := connector.GetProducts(ctx, domain.ProductFilter{Search: line.ProductName, Limit: 5})
				if err == nil && len(prods) > 0 {
					names := make([]string, 0, len(prods))
					skus := make([]string, 0, len(prods))
					ids := make([]string, 0, len(prods))
					for _, p := range prods {
						names = append(names, p.Name)
						skus = append(skus, p.SKU)
						ids = append(ids, p.ID)
					}
					picked, perr := resolveProductID(line.ProductName, names, skus, ids)
					if perr != nil {
						return "", nil, perr
					}
					prodID = picked
				}
			}
			if prodID != "" {
				lineReqs = append(lineReqs, domain.CreateSaleOrderLineRequest{
					ProductID: prodID,
					Quantity:  line.Quantity,
					UnitPrice: line.UnitPrice,
				})
			}
		}
	}

	if len(lineReqs) == 0 {
		prodID := params.ProductID
		if prodID == "" && params.ProductName != "" {
			prods, err := connector.GetProducts(ctx, domain.ProductFilter{Search: params.ProductName, Limit: 5})
			if err == nil && len(prods) > 0 {
				names := make([]string, 0, len(prods))
				skus := make([]string, 0, len(prods))
				ids := make([]string, 0, len(prods))
				for _, p := range prods {
					names = append(names, p.Name)
					skus = append(skus, p.SKU)
					ids = append(ids, p.ID)
				}
				picked, perr := resolveProductID(params.ProductName, names, skus, ids)
				if perr != nil {
					return "", nil, perr
				}
				prodID = picked
			}
		}
		if prodID == "" {
			if params.ProductName != "" {
				return "", nil, fmt.Errorf("could not find product matching: %s", params.ProductName)
			}
			return "", nil, fmt.Errorf("product ID or Name is required")
		}
		qty := params.Quantity
		if qty <= 0 {
			qty = 1.0
		}
		lineReqs = append(lineReqs, domain.CreateSaleOrderLineRequest{
			ProductID: prodID,
			Quantity:  qty,
			UnitPrice: params.UnitPrice,
		})
	}

	return custID, lineReqs, nil
}
