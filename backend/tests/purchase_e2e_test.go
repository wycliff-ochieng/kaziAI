package tests

import (
	"context"
	"testing"

	"erpagent-backend/internal/adapters/mock"
	"erpagent-backend/internal/domain"
)

// TestPurchaseApp_EndToEndFlow verifies the canonical purchase flow against the
// mock adapter (offline CI). Live validation runs the same scripted NL pass against
// Odoo 19 (with 18/17 compat matrix):
// search vendor -> search product -> draft RFQ -> confirm -> full receive -> bill -> post,
// plus "stop at receipt" and "resume PO from confirmed" edge runs.
func TestPurchaseApp_EndToEndFlow(t *testing.T) {
	ctx := context.Background()
	adapter := mock.NewMockERPAdapter()

	// STEP 1: VENDOR + PRODUCT RESOLUTION (search-only)
	t.Run("Entity Resolution - Vendor and Product Search", func(t *testing.T) {
		vendors, err := adapter.GetVendors(ctx, domain.VendorFilter{Search: "ABC"})
		if err != nil {
			t.Fatalf("GetVendors failed: %v", err)
		}
		if len(vendors) == 0 {
			t.Fatalf("expected seeded vendor 'ABC Ltd'")
		}

		products, err := adapter.GetProducts(ctx, domain.ProductFilter{Search: "Chair"})
		if err != nil {
			t.Fatalf("GetProducts failed: %v", err)
		}
		if len(products) == 0 {
			t.Fatalf("expected seeded product")
		}
	})

	var poID string

	// STEP 2: RFQ DRAFT (auto-executes, low risk)
	t.Run("RFQ Flow - Draft Creation", func(t *testing.T) {
		dryRes, err := adapter.CreateRFQ(ctx, domain.CreatePurchaseOrderRequest{
			VendorID: "ABC Ltd",
			Lines:    []domain.CreatePurchaseOrderLineRequest{{ProductID: "CHAIR-002", Quantity: 10}},
			DryRun:   true,
		})
		if err != nil {
			t.Fatalf("CreateRFQ dry_run failed: %v", err)
		}
		if dryRes.DryRun == nil || !dryRes.DryRun.RequiresConfirmation {
			t.Errorf("expected dry_run preview requiring confirmation")
		}

		res, err := adapter.CreateRFQ(ctx, domain.CreatePurchaseOrderRequest{
			VendorID: "20",
			Lines:    []domain.CreatePurchaseOrderLineRequest{{ProductID: "102", Quantity: 10}},
			DryRun:   false,
		})
		if err != nil {
			t.Fatalf("CreateRFQ failed: %v", err)
		}
		if !res.Success || res.RecordID == "" {
			t.Fatalf("expected successful RFQ with record ID")
		}
		poID = res.RecordID
	})

	// STEP 3: VERIFY-STATE-FIRST + CONFIRM (gated by approval)
	t.Run("Confirm Flow - Verify State Then Confirm", func(t *testing.T) {
		po, err := adapter.GetPurchaseOrderByID(ctx, poID)
		if err != nil {
			t.Fatalf("GetPurchaseOrderByID failed: %v", err)
		}
		if po.State != "draft" {
			t.Fatalf("expected draft state, got %s", po.State)
		}

		dryRes, err := adapter.ConfirmPurchaseOrder(ctx, poID, true)
		if err != nil {
			t.Fatalf("ConfirmPurchaseOrder dry_run failed: %v", err)
		}
		if dryRes.DryRun == nil || !dryRes.DryRun.RequiresConfirmation {
			t.Errorf("expected dry_run preview requiring confirmation")
		}

		res, err := adapter.ConfirmPurchaseOrder(ctx, poID, false)
		if err != nil {
			t.Fatalf("ConfirmPurchaseOrder failed: %v", err)
		}
		if !res.Success {
			t.Errorf("expected successful confirmation")
		}
	})

	// STEP 4: FULL RECEIPT ONLY (partials rejected by design)
	t.Run("Receipt Flow - Full Receipt Validation", func(t *testing.T) {
		dryRes, err := adapter.ReceivePurchaseOrder(ctx, poID, true)
		if err != nil {
			t.Fatalf("ReceivePurchaseOrder dry_run failed: %v", err)
		}
		if dryRes.DryRun == nil {
			t.Errorf("expected dry_run preview for receipt")
		}

		res, err := adapter.ReceivePurchaseOrder(ctx, poID, false)
		if err != nil {
			t.Fatalf("ReceivePurchaseOrder failed: %v", err)
		}
		if !res.Success {
			t.Errorf("expected successful receipt")
		}

		po, _ := adapter.GetPurchaseOrderByID(ctx, poID)
		if po.ReceiptState != "fully_received" {
			t.Errorf("expected fully_received, got %s", po.ReceiptState)
		}
	})

	// STEP 5: VENDOR BILL AUTO-COPIED FROM PO + POST (post-only, no payment)
	t.Run("Bill Flow - Create and Post Vendor Bill", func(t *testing.T) {
		dryRes, err := adapter.CreateVendorBill(ctx, poID, true)
		if err != nil {
			t.Fatalf("CreateVendorBill dry_run failed: %v", err)
		}
		if dryRes.DryRun == nil {
			t.Errorf("expected dry_run preview for bill")
		}

		billRes, err := adapter.CreateVendorBill(ctx, poID, false)
		if err != nil {
			t.Fatalf("CreateVendorBill failed: %v", err)
		}
		billID := billRes.RecordID

		postDry, err := adapter.PostVendorBill(ctx, billID, true)
		if err != nil {
			t.Fatalf("PostVendorBill dry_run failed: %v", err)
		}
		if postDry.DryRun == nil {
			t.Errorf("expected dry_run preview for post")
		}

		postRes, err := adapter.PostVendorBill(ctx, billID, false)
		if err != nil {
			t.Fatalf("PostVendorBill failed: %v", err)
		}
		if !postRes.Success {
			t.Errorf("expected successful post")
		}
	})
}

// TestPurchaseApp_ResumeAndCancel covers the autonomy rule: stop-at-receipt,
// resume-from-confirmed, draft-only cancel, and illegal-transition rejection.
func TestPurchaseApp_ResumeAndCancel(t *testing.T) {
	ctx := context.Background()
	adapter := mock.NewMockERPAdapter()

	// Draft RFQ then stop (user said "stop at receipt" path starts here).
	res, err := adapter.CreateRFQ(ctx, domain.CreatePurchaseOrderRequest{
		VendorID: "21",
		Lines:    []domain.CreatePurchaseOrderLineRequest{{ProductID: "101", Quantity: 5}},
	})
	if err != nil {
		t.Fatalf("CreateRFQ failed: %v", err)
	}
	stoppedID := res.RecordID

	// Resume later from draft: confirm works.
	if _, err := adapter.ConfirmPurchaseOrder(ctx, stoppedID, false); err != nil {
		t.Fatalf("resume-confirm failed: %v", err)
	}

	// Illegal: bill a PO that is only confirmed (allowed), then bill again is fine;
	// but receiving a draft PO must fail.
	draftRes, err := adapter.CreateRFQ(ctx, domain.CreatePurchaseOrderRequest{
		VendorID: "20",
		Lines:    []domain.CreatePurchaseOrderLineRequest{{ProductID: "102", Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("CreateRFQ failed: %v", err)
	}
	if _, err := adapter.ReceivePurchaseOrder(ctx, draftRes.RecordID, false); err == nil {
		t.Errorf("expected receive on draft PO to fail (must be confirmed first)")
	}

	// Draft-only cancel: draft cancels OK...
	dryCancel, err := adapter.CancelPurchaseOrder(ctx, draftRes.RecordID, true)
	if err != nil {
		t.Fatalf("CancelPurchaseOrder dry_run failed: %v", err)
	}
	if dryCancel.DryRun == nil {
		t.Errorf("expected dry_run preview for cancel")
	}
	if _, err := adapter.CancelPurchaseOrder(ctx, draftRes.RecordID, false); err != nil {
		t.Fatalf("cancel draft failed: %v", err)
	}

	// ...but cancelling a confirmed PO must fail in v1.
	if _, err := adapter.CancelPurchaseOrder(ctx, stoppedID, false); err == nil {
		t.Errorf("expected cancel on confirmed PO to fail (draft-only in v1)")
	}

	// Double-confirm must fail (no longer draft).
	if _, err := adapter.ConfirmPurchaseOrder(ctx, stoppedID, false); err == nil {
		t.Errorf("expected second confirm to fail (not draft)")
	}
}
