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
	"erpagent-backend/internal/factory"
	"erpagent-backend/middleware"
)

type ChatRequest struct {
	Prompt   string `json:"prompt"`
	TenantID string `json:"tenant_id,omitempty"`
	ERPType  string `json:"erp_type,omitempty"`
}

type OrderLinePayload struct {
	ProductName string  `json:"product_name"`
	ProductID   string  `json:"product_id"`
	Quantity    float64 `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
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

		// 1. Call AI Engine for intent parsing
		parsedIntent, err := callAIEngine(r.Context(), cfg.AIEngineURL, req.Prompt, tenantID)
		if err != nil {
			RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"error":   "AI Engine communication error",
				"details": err.Error(),
			})
			return
		}

		// 2. Resolve ERP Connector
		erpCfg := factory.TenantConfig{
			TenantID: tenantID,
			ERPType:  req.ERPType,
			BaseURL:  cfg.OdooURL,
			DBName:   cfg.OdooDB,
			Username: cfg.OdooUser,
			Password: cfg.OdooPassword,
		}

		connector, err := factory.GetERPConnector(erpCfg)
		if err != nil {
			RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"error":   "Failed to initialize ERP Connector",
				"details": err.Error(),
			})
			return
		}

		// 3. Execute canonical ERP operation based on classified intent
		response, err := executeIntent(r.Context(), connector, req.Prompt, parsedIntent)
		if err != nil {
			RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
				"error":         "ERP action execution failed",
				"details":       err.Error(),
				"parsed_intent": parsedIntent,
			})
			return
		}

		RespondJSON(w, http.StatusOK, response)
	}
}

func callAIEngine(ctx context.Context, aiURL, prompt, tenantID string) (*ParsedIntentResponse, error) {
	reqPayload := map[string]interface{}{
		"prompt":    prompt,
		"tenant_id": tenantID,
	}

	body, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, err
	}

	parseEndpoint := fmt.Sprintf("%s/api/v1/parse-intent", strings.TrimSuffix(aiURL, "/"))
	req, err := http.NewRequestWithContext(ctx, "POST", parseEndpoint, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Close = true // Disable HTTP Keep-Alive persistence for test clean-up

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http call to AI Engine failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AI Engine returned error code %d: %s", resp.StatusCode, string(respBytes))
	}

	var parsed ParsedIntentResponse
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return nil, fmt.Errorf("failed to unmarshal AI Engine response: %w", err)
	}

	return &parsed, nil
}

func executeIntent(ctx context.Context, connector domain.ERPConnector, prompt string, parsed *ParsedIntentResponse) (*ChatResponse, error) {
	params := parsed.Parameters

	switch parsed.IntentType {

	case "GET_CUSTOMERS":
		filter := domain.CustomerFilter{Search: params.Search}
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
		filter := domain.ProductFilter{Search: params.Search}
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
		filter := domain.SaleOrderFilter{State: params.State, Search: params.Search}
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
		invoices, err := connector.GetInvoices(ctx, domain.InvoiceFilter{})
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
		vendors, err := connector.GetVendors(ctx, domain.VendorFilter{Search: params.Search})
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
		filter := domain.PurchaseOrderFilter{State: params.State, Search: params.Search}
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

	default:
		return &ChatResponse{
			Status:       "unknown_intent",
			Prompt:       prompt,
			ParsedIntent: parsed,
			Message:      fmt.Sprintf("I understood this as a %s request, but I lack the tools to execute it.", parsed.IntentType),
		}, nil
	}
}

func resolvePurchaseParams(ctx context.Context, connector domain.ERPConnector, params IntentParameters) (string, []domain.CreatePurchaseOrderLineRequest, error) {
	vendorID := params.VendorID
	if vendorID == "" && params.VendorName != "" {
		vendors, err := connector.GetVendors(ctx, domain.VendorFilter{Search: params.VendorName})
		if err == nil && len(vendors) > 0 {
			vendorID = vendors[0].ID
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
				prods, err := connector.GetProducts(ctx, domain.ProductFilter{Search: line.ProductName})
				if err == nil && len(prods) > 0 {
					prodID = prods[0].ID
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
			prods, err := connector.GetProducts(ctx, domain.ProductFilter{Search: params.ProductName})
			if err == nil && len(prods) > 0 {
				prodID = prods[0].ID
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
		custs, err := connector.GetCustomers(ctx, domain.CustomerFilter{Search: params.CustomerName})
		if err == nil && len(custs) > 0 {
			custID = custs[0].ID
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
				prods, err := connector.GetProducts(ctx, domain.ProductFilter{Search: line.ProductName})
				if err == nil && len(prods) > 0 {
					prodID = prods[0].ID
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
			prods, err := connector.GetProducts(ctx, domain.ProductFilter{Search: params.ProductName})
			if err == nil && len(prods) > 0 {
				prodID = prods[0].ID
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
