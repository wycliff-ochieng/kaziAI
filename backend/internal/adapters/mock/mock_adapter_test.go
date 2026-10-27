package mock

import (
	"context"
	"strings"
	"testing"

	"erpagent-backend/internal/domain"
)

func TestMockAdapter_SeedDataServesCatalogThroughPublicInterface(t *testing.T) {
	a := NewMockERPAdapter()
	ctx := context.Background()

	customers, err := a.GetCustomers(ctx, domain.CustomerFilter{})
	if err != nil {
		t.Fatalf("GetCustomers failed: %v", err)
	}
	if len(customers) != 3 {
		t.Fatalf("expected 3 seeded customers, got %d", len(customers))
	}

	products, err := a.GetProducts(ctx, domain.ProductFilter{})
	if err != nil {
		t.Fatalf("GetProducts failed: %v", err)
	}
	if len(products) != 3 {
		t.Fatalf("expected 3 seeded products, got %d", len(products))
	}

	order, err := a.GetSaleOrderByID(ctx, "SO001")
	if err != nil {
		t.Fatalf("GetSaleOrderByID SO001 failed: %v", err)
	}
	if order.CustomerName != "Azure Interior" {
		t.Errorf("expected Azure Interior, got %s", order.CustomerName)
	}
}

func TestMockAdapter_CreateSaleOrderDryRunDoesNotPersist(t *testing.T) {
	a := NewMockERPAdapter()
	ctx := context.Background()

	before, _ := a.GetSaleOrders(ctx, domain.SaleOrderFilter{})
	res, err := a.CreateSaleOrder(ctx, domain.CreateSaleOrderRequest{
		CustomerID: "10",
		Lines:      []domain.CreateSaleOrderLineRequest{{ProductID: "101", Quantity: 2}},
		DryRun:     true,
	})
	if err != nil {
		t.Fatalf("dry-run create failed: %v", err)
	}
	if res.DryRun == nil || !res.DryRun.IsDryRun {
		t.Fatalf("expected dry-run payload")
	}
	if !res.DryRun.RequiresConfirmation {
		t.Errorf("expected RequiresConfirmation true")
	}
	after, _ := a.GetSaleOrders(ctx, domain.SaleOrderFilter{})
	if len(after) != len(before) {
		t.Errorf("dry-run must not persist orders: before %d after %d", len(before), len(after))
	}
}

func TestMockAdapter_CreateSaleOrderLivePersists(t *testing.T) {
	a := NewMockERPAdapter()
	ctx := context.Background()

	res, err := a.CreateSaleOrder(ctx, domain.CreateSaleOrderRequest{
		CustomerID: "10",
		Lines:      []domain.CreateSaleOrderLineRequest{{ProductID: "101", Quantity: 1}},
		DryRun:     false,
	})
	if err != nil {
		t.Fatalf("live create failed: %v", err)
	}
	if !res.Success || res.RecordID == "" {
		t.Fatalf("expected successful live create with record ID")
	}
	got, err := a.GetSaleOrderByID(ctx, res.RecordID)
	if err != nil {
		t.Fatalf("created order not retrievable: %v", err)
	}
	if got.State != "draft" {
		t.Errorf("expected draft state, got %s", got.State)
	}
}

func TestMockAdapter_ConfirmAndDeliverLifecycle(t *testing.T) {
	a := NewMockERPAdapter()
	ctx := context.Background()

	dry, err := a.ConfirmSaleOrder(ctx, "SO001", true)
	if err != nil {
		t.Fatalf("confirm dry-run failed: %v", err)
	}
	if dry.DryRun == nil || dry.DryRun.RiskLevel != domain.RiskMedium {
		t.Errorf("expected MEDIUM risk dry-run for confirm")
	}

	live, err := a.ConfirmSaleOrder(ctx, "SO001", false)
	if err != nil {
		t.Fatalf("confirm live failed: %v", err)
	}
	if !live.Success {
		t.Fatalf("expected confirm success")
	}
	order, _ := a.GetSaleOrderByID(ctx, "SO001")
	if order.State != "sale" {
		t.Errorf("expected sale state after confirm, got %s", order.State)
	}

	deliver, err := a.DeliverSaleOrder(ctx, "SO001", false)
	if err != nil {
		t.Fatalf("deliver failed: %v", err)
	}
	if !deliver.Success {
		t.Fatalf("expected deliver success")
	}
	done, _ := a.GetSaleOrderByID(ctx, "SO001")
	if done.State != "done" {
		t.Errorf("expected done state after deliver, got %s", done.State)
	}
}

func TestMockAdapter_CreditNoteAndETIMSAndPrint(t *testing.T) {
	a := NewMockERPAdapter()
	ctx := context.Background()

	cn, err := a.CreateCreditNote(ctx, domain.CreditNoteRequest{
		InvoiceID: "INV/2026/0001",
		Reason:    "damaged goods",
		Amount:    200,
		DryRun:    false,
	})
	if err != nil {
		t.Fatalf("credit note failed: %v", err)
	}
	if !cn.Success {
		t.Errorf("expected credit note success")
	}

	etims, err := a.SubmitETIMS(ctx, "INV/2026/0001", false)
	if err != nil {
		t.Fatalf("etims failed: %v", err)
	}
	if etims.FiscalStatus != "SUCCESS" || etims.CUSerialNo == "" || etims.IRN == "" {
		t.Errorf("expected SUCCESS with CU serial and IRN, got %+v", etims)
	}

	printed, err := a.PrintDocument(ctx, domain.PrintDocumentRequest{
		DocumentType: "quotation",
		DocumentID:   "SO001",
		Format:       "pdf",
	})
	if err != nil {
		t.Fatalf("print failed: %v", err)
	}
	if printed.DownloadURL == "" {
		t.Errorf("expected download URL for printed document")
	}
}

func TestMockAdapter_MissingRecordsReturnErrors(t *testing.T) {
	a := NewMockERPAdapter()
	ctx := context.Background()

	if _, err := a.GetCustomerByID(ctx, "no-such-customer"); err == nil {
		t.Errorf("expected error for missing customer")
	}
	if _, err := a.GetSaleOrderByID(ctx, "NOPE"); err == nil {
		t.Errorf("expected error for missing order")
	}
	if _, err := a.CreateCreditNote(ctx, domain.CreditNoteRequest{InvoiceID: "NOPE"}); err == nil {
		t.Errorf("expected error for credit note on missing invoice")
	}
}

func TestMockAdapter_DryRunDoesNotConsumeSequence(t *testing.T) {
	a := NewMockERPAdapter()
	ctx := context.Background()
	req := domain.CreatePurchaseOrderRequest{
		VendorID: "20",
		Lines:    []domain.CreatePurchaseOrderLineRequest{{ProductID: "101", Quantity: 1}},
	}

	req.DryRun = true
	preview, err := a.CreateRFQ(ctx, req)
	if err != nil {
		t.Fatalf("dry-run RFQ failed: %v", err)
	}
	if preview.DryRun == nil {
		t.Fatalf("expected dry-run payload")
	}

	req.DryRun = false
	live, err := a.CreateRFQ(ctx, req)
	if err != nil {
		t.Fatalf("live RFQ failed: %v", err)
	}
	if !strings.Contains(preview.DryRun.ImpactSummary, live.RecordNumber) {
		t.Fatalf("preview %q does not match live number %q", preview.DryRun.ImpactSummary, live.RecordNumber)
	}
}
