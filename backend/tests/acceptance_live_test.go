package tests

import (
	"context"
	"os"
	"testing"

	"erpagent-backend/internal/adapters/odoo"
	"erpagent-backend/internal/domain"
)

// TestAcceptance_LiveOdoo is the executable live runbook (locked Q10/Q12).
//
// Read-only by default: version probe (must be 17/18/19), vendor/product/PO
// search, RFQ dry-run, confirm dry-run on an existing draft PO if one exists.
// No mutation unless ODOO_LIVE_WRITE=1, in which case it runs the full
// RFQ -> confirm -> receive -> bill -> post chain (qty 1) and logs every
// record ID for manual cleanup.
//
//	ODOO_LIVE_URL=https://odoo.example.com ODOO_LIVE_DB=prod \
//	ODOO_LIVE_USER=api ODOO_LIVE_PASSWORD=secret go test ./tests/ -run TestAcceptance_LiveOdoo -v
func TestAcceptance_LiveOdoo(t *testing.T) {
	baseURL := os.Getenv("ODOO_LIVE_URL")
	if baseURL == "" {
		t.Skip("ODOO_LIVE_URL unset: live runbook skipped (mock CI covers the flow)")
	}
	ctx := context.Background()
	adapter := odoo.NewOdooAdapter(
		baseURL,
		os.Getenv("ODOO_LIVE_DB"),
		os.Getenv("ODOO_LIVE_USER"),
		os.Getenv("ODOO_LIVE_PASSWORD"),
	)

	// STEP 1: version probe must report a supported major.
	v := adapter.DetectVersion(ctx)
	t.Logf("live Odoo version: %s", v.Full)
	if v.Major != 17 && v.Major != 18 && v.Major != 19 {
		t.Fatalf("unsupported live major %d (want 17/18/19)", v.Major)
	}

	// STEP 2: read paths.
	vendors, err := adapter.GetVendors(ctx, domain.VendorFilter{Limit: 5})
	if err != nil {
		t.Fatalf("live GetVendors failed: %v", err)
	}
	t.Logf("live vendors visible: %d", len(vendors))

	products, err := adapter.GetProducts(ctx, domain.ProductFilter{Limit: 5})
	if err != nil {
		t.Fatalf("live GetProducts failed: %v", err)
	}
	if len(products) == 0 {
		t.Fatalf("live catalog empty: cannot proceed")
	}

	drafts, err := adapter.GetPurchaseOrders(ctx, domain.PurchaseOrderFilter{State: "draft", Limit: 5})
	if err != nil {
		t.Fatalf("live GetPurchaseOrders failed: %v", err)
	}
	t.Logf("live draft RFQs visible: %d", len(drafts))

	if len(vendors) == 0 {
		t.Fatalf("no live vendors: cannot dry-run RFQ")
	}

	// STEP 3: RFQ dry-run (no mutation) against the first live vendor/product.
	dry, err := adapter.CreateRFQ(ctx, domain.CreatePurchaseOrderRequest{
		VendorID: vendors[0].ID,
		Lines:    []domain.CreatePurchaseOrderLineRequest{{ProductID: products[0].ID, Quantity: 1}},
		DryRun:   true,
	})
	if err != nil {
		t.Fatalf("live RFQ dry-run failed: %v", err)
	}
	if dry.DryRun == nil || !dry.DryRun.RequiresConfirmation {
		t.Fatalf("expected dry-run preview requiring confirmation")
	}
	t.Logf("live RFQ dry-run OK: %s", dry.Message)

	// STEP 4: confirm dry-run on an existing draft (no mutation).
	if len(drafts) > 0 {
		if _, err := adapter.ConfirmPurchaseOrder(ctx, drafts[0].ID, true); err != nil {
			t.Fatalf("live confirm dry-run failed: %v", err)
		}
		t.Logf("live confirm dry-run OK on %s", drafts[0].OrderNo)
	} else {
		t.Log("no live drafts: confirm dry-run skipped")
	}

	// STEP 5: full write chain, strictly opt-in.
	if os.Getenv("ODOO_LIVE_WRITE") != "1" {
		t.Log("ODOO_LIVE_WRITE!=1: write chain skipped (read-only pass complete)")
		return
	}

	created, err := adapter.CreateRFQ(ctx, domain.CreatePurchaseOrderRequest{
		VendorID: vendors[0].ID,
		Lines:    []domain.CreatePurchaseOrderLineRequest{{ProductID: products[0].ID, Quantity: 1}},
		Note:     "ERPAgent acceptance runbook (qty 1) - safe to cancel/posted bill needs manual review",
	})
	if err != nil {
		t.Fatalf("live RFQ create failed: %v", err)
	}
	poID := created.RecordID
	t.Logf("LIVE RFQ: %s (manual cleanup if later steps fail)", created.RecordNumber)

	if _, err := adapter.ConfirmPurchaseOrder(ctx, poID, false); err != nil {
		t.Fatalf("live confirm failed (PO %s needs manual review): %v", poID, err)
	}
	if _, err := adapter.ReceivePurchaseOrder(ctx, poID, false); err != nil {
		t.Fatalf("live receive failed (PO %s needs manual review): %v", poID, err)
	}
	bill, err := adapter.CreateVendorBill(ctx, poID, false)
	if err != nil {
		t.Fatalf("live bill failed (PO %s needs manual review): %v", poID, err)
	}
	if _, err := adapter.PostVendorBill(ctx, bill.RecordID, false); err != nil {
		t.Fatalf("live post failed (bill %s needs manual review): %v", bill.RecordID, err)
	}
	t.Logf("LIVE CHAIN COMPLETE: PO %s -> bill %s posted", poID, bill.RecordNumber)
}
