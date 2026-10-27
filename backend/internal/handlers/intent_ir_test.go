package handlers

import (
	"context"
	"strings"
	"testing"

	"erpagent-backend/internal/domain"
)

// captureConnector embeds the full ERP interface and overrides only the calls a
// test cares about, recording the filters the executor actually passes down.
type captureConnector struct {
	domain.ERPConnector

	customerFilter domain.CustomerFilter
	invoiceFilter  domain.InvoiceFilter
	saleFilter     domain.SaleOrderFilter

	customers []domain.Customer

	confirmOrderID string
	deliverOrderID string
	createData     any
	createDryRun   *domain.DryRunResult
	customerCalls  int
}

func (c *captureConnector) GetCustomers(_ context.Context, f domain.CustomerFilter) ([]domain.Customer, error) {
	c.customerFilter = f
	c.customerCalls++
	return c.customers, nil
}

func (c *captureConnector) GetInvoices(_ context.Context, f domain.InvoiceFilter) ([]domain.Invoice, error) {
	c.invoiceFilter = f
	return []domain.Invoice{{ID: "201", InvoiceNo: "INV/1", Status: f.Status}}, nil
}

func (c *captureConnector) GetSaleOrders(_ context.Context, f domain.SaleOrderFilter) ([]domain.SaleOrder, error) {
	c.saleFilter = f
	return []domain.SaleOrder{{ID: "501", OrderNo: "SO001", State: f.State}}, nil
}

func (c *captureConnector) CreateSaleOrder(_ context.Context, _ domain.CreateSaleOrderRequest) (*domain.ActionResult, error) {
	return &domain.ActionResult{Success: true, Message: "created", DryRun: c.createDryRun, Data: c.createData}, nil
}

func (c *captureConnector) ConfirmSaleOrder(_ context.Context, orderID string, _ bool) (*domain.ActionResult, error) {
	c.confirmOrderID = orderID
	return &domain.ActionResult{Success: true, Message: "confirmed " + orderID}, nil
}

func (c *captureConnector) DeliverSaleOrder(_ context.Context, orderID string, _ bool) (*domain.ActionResult, error) {
	c.deliverOrderID = orderID
	return &domain.ActionResult{Success: true, Message: "delivered " + orderID}, nil
}

func TestExecuteIntent_PassesLimitAndDomainToCustomers(t *testing.T) {
	conn := &captureConnector{}
	parsed := &ParsedIntentResponse{
		IntentType: "GET_CUSTOMERS",
		ActionType: "READ",
		Parameters: IntentParameters{
			Limit:  10,
			Domain: []any{[]any{"sale_order_count", ">", 1}},
		},
	}
	if _, err := executeIntent(context.Background(), conn, "first 10 customers with orders", parsed); err != nil {
		t.Fatalf("executeIntent: %v", err)
	}
	if conn.customerFilter.Limit != 10 {
		t.Errorf("limit not forwarded: %+v", conn.customerFilter)
	}
	if len(conn.customerFilter.Domain) != 1 {
		t.Errorf("domain not forwarded: %+v", conn.customerFilter.Domain)
	}
}

func TestExecuteIntent_PassesInvoiceStateAndLimit(t *testing.T) {
	conn := &captureConnector{}
	parsed := &ParsedIntentResponse{
		IntentType: "GET_INVOICES",
		ActionType: "READ",
		Parameters: IntentParameters{State: "posted", Limit: 5},
	}
	if _, err := executeIntent(context.Background(), conn, "paid invoices", parsed); err != nil {
		t.Fatalf("executeIntent: %v", err)
	}
	if conn.invoiceFilter.Status != "posted" || conn.invoiceFilter.Limit != 5 {
		t.Errorf("invoice filter not forwarded: %+v", conn.invoiceFilter)
	}
}

func TestExecuteIntent_ConversationalNeverCallsConnector(t *testing.T) {
	conn := &captureConnector{}
	parsed := &ParsedIntentResponse{
		IntentType: "CONVERSATION",
		ActionType: "CONVERSATION",
		Summary:    "Hello! I'm your Odoo assistant.",
		Parameters: IntentParameters{DryRun: false},
	}
	resp, err := executeIntent(context.Background(), conn, "hello", parsed)
	if err != nil {
		t.Fatalf("executeIntent: %v", err)
	}
	if resp.Message != parsed.Summary {
		t.Errorf("expected canned summary, got %q", resp.Message)
	}
	if conn.customerCalls != 0 {
		t.Errorf("conversational turn must not query the ERP")
	}
}

func TestExecuteIntent_CountRecords(t *testing.T) {
	conn := &captureConnector{customers: []domain.Customer{{ID: "1"}, {ID: "2"}, {ID: "3"}}}
	parsed := &ParsedIntentResponse{
		IntentType: "COUNT_RECORDS",
		ActionType: "READ",
		Parameters: IntentParameters{Resource: "customers"},
	}
	resp, err := executeIntent(context.Background(), conn, "how many customers", parsed)
	if err != nil {
		t.Fatalf("executeIntent: %v", err)
	}
	if !strings.Contains(resp.Message, "3") {
		t.Errorf("expected a count of 3, got %q", resp.Message)
	}
}

func TestExecuteIntent_UnknownIntentSuggestsRetry(t *testing.T) {
	conn := &captureConnector{}
	parsed := &ParsedIntentResponse{IntentType: "SOMETHING_ELSE", ActionType: "READ"}
	resp, _ := executeIntent(context.Background(), conn, "whatever", parsed)
	if !strings.Contains(resp.Message, "Could not map") {
		t.Errorf("unhelpful unknown-intent message: %q", resp.Message)
	}
}

func TestExecuteIntent_ChainThreadsCreatedOrderID(t *testing.T) {
	conn := &captureConnector{createData: domain.SaleOrder{ID: "777", OrderNo: "SO777"}}
	parsed := &ParsedIntentResponse{
		IntentType: "CREATE_SALE_ORDER",
		ActionType: "WRITE",
		Parameters: IntentParameters{
			CustomerID: "10",
			Lines:      []OrderLinePayload{{ProductID: "101", Quantity: 1}},
			DryRun:     false,
		},
		Steps: []IntentStep{
			{IntentType: "CONFIRM_SALE_ORDER", Summary: "confirm"},
			{IntentType: "DELIVER_SALE_ORDER", Summary: "deliver"},
		},
	}
	if _, err := executeIntent(context.Background(), conn, "create and confirm", parsed); err != nil {
		t.Fatalf("executeIntent: %v", err)
	}
	if conn.confirmOrderID != "777" {
		t.Errorf("chain did not thread the created order id, got %q", conn.confirmOrderID)
	}
}

func TestExecuteIntent_ChainHeldWhenDryRun(t *testing.T) {
	conn := &captureConnector{
		createData:   domain.SaleOrder{ID: "777"},
		createDryRun: &domain.DryRunResult{IsDryRun: true, Operation: "CREATE_SALE_ORDER", RequiresConfirmation: true},
	}
	parsed := &ParsedIntentResponse{
		IntentType: "CREATE_SALE_ORDER",
		ActionType: "WRITE",
		Parameters: IntentParameters{
			CustomerID: "10",
			Lines:      []OrderLinePayload{{ProductID: "101", Quantity: 1}},
			DryRun:     true,
		},
		Steps: []IntentStep{{IntentType: "CONFIRM_SALE_ORDER", Summary: "confirm"}},
	}
	resp, err := executeIntent(context.Background(), conn, "create SO", parsed)
	if err != nil {
		t.Fatalf("executeIntent: %v", err)
	}
	if conn.confirmOrderID != "" {
		t.Errorf("held dry-run must not execute follow-on steps")
	}
	if !strings.Contains(resp.Message, "Planned next") {
		t.Errorf("expected planned-steps note, got %q", resp.Message)
	}
}

func TestResolveOrderParams_AmbiguousCustomerIsRejected(t *testing.T) {
	conn := &captureConnector{customers: []domain.Customer{
		{ID: "146", Name: "Administrator"},
		{ID: "176", Name: "Administrator"},
	}}
	_, _, err := resolveOrderParams(context.Background(), conn, IntentParameters{
		CustomerName: "Administrator",
		ProductID:    "101",
		Quantity:     1,
	})
	if err == nil || !strings.Contains(err.Error(), "multiple") {
		t.Fatalf("expected ambiguity error, got %v", err)
	}
}
