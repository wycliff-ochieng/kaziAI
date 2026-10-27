package odoo

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"erpagent-backend/internal/domain"
)

// Purchase mapping notes (Odoo 17/18/19):
// - Vendors live in res.partner with supplier_rank >= 1.
// - RFQs/POs live in purchase.order (states: draft, sent, to approve, purchase, done, cancel).
// - Lines live in purchase.order.line (product_id, product_qty, price_unit, price_subtotal).
// - Confirm uses button_confirm (stable across 17/18/19); cancel uses button_cancel.
// - Receipts are inbound stock.picking records linked via purchase_id; validated with
//   action_assign + action_set_quantities_to_reservation + button_validate (full receipt only in v1).
// - Vendor bills are account.move records with move_type in_invoice, created from a PO via
//   purchase.order action_create_invoice, posted via account.move action_post.

func (o *OdooAdapter) GetVendors(ctx context.Context, filter domain.VendorFilter) ([]domain.Vendor, error) {
	domainFilter := []interface{}{
		[]interface{}{"supplier_rank", ">=", 1},
	}
	if filter.Search != "" {
		domainFilter = append(domainFilter, []interface{}{"name", "ilike", filter.Search})
	}
	if filter.Email != "" {
		domainFilter = append(domainFilter, []interface{}{"email", "=", filter.Email})
	}
	domainFilter = appendExtraDomain(domainFilter, filter.Domain)

	limit := 100
	if filter.Limit > 0 {
		limit = filter.Limit
	}

	kwargs := map[string]interface{}{
		"fields": []string{"id", "name", "email", "phone", "street", "city", "country_id", "total_due"},
		"limit":  limit,
		"offset": filter.Offset,
	}

	resRaw, err := o.executeKW(ctx, "res.partner", "search_read", []interface{}{domainFilter}, kwargs)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch vendors from Odoo: %w", err)
	}

	var items []struct {
		ID       int         `json:"id"`
		Name     string      `json:"name"`
		Email    interface{} `json:"email"`
		Phone    interface{} `json:"phone"`
		Street   interface{} `json:"street"`
		City     interface{} `json:"city"`
		Country  any         `json:"country_id"`
		TotalDue float64     `json:"total_due"`
	}
	if err := json.Unmarshal(resRaw, &items); err != nil {
		return nil, fmt.Errorf("failed to parse Odoo vendors response: %w", err)
	}

	vendors := make([]domain.Vendor, 0, len(items))
	for _, item := range items {
		_, country := parseMany2One(item.Country)
		vendors = append(vendors, domain.Vendor{
			ID:      fmt.Sprintf("%d", item.ID),
			Name:    fmtStr(item.Name),
			Email:   fmtStr(item.Email),
			Phone:   fmtStr(item.Phone),
			Street:  fmtStr(item.Street),
			City:    fmtStr(item.City),
			Country: country,
			Balance: item.TotalDue,
		})
	}

	return vendors, nil
}

func (o *OdooAdapter) GetVendorByID(ctx context.Context, id string) (*domain.Vendor, error) {
	idInt, err := strconv.Atoi(id)
	if err != nil {
		return nil, fmt.Errorf("invalid vendor id format: %s", id)
	}

	domainFilter := []interface{}{
		[]interface{}{"id", "=", idInt},
	}
	kwargs := map[string]interface{}{
		"fields": []string{"id", "name", "email", "phone", "street", "city", "country_id", "total_due"},
		"limit":  1,
	}

	resRaw, err := o.executeKW(ctx, "res.partner", "search_read", []interface{}{domainFilter}, kwargs)
	if err != nil {
		return nil, err
	}

	var items []struct {
		ID       int         `json:"id"`
		Name     string      `json:"name"`
		Email    interface{} `json:"email"`
		Phone    interface{} `json:"phone"`
		Street   interface{} `json:"street"`
		City     interface{} `json:"city"`
		Country  any         `json:"country_id"`
		TotalDue float64     `json:"total_due"`
	}
	if err := json.Unmarshal(resRaw, &items); err != nil || len(items) == 0 {
		return nil, fmt.Errorf("vendor not found with ID %s", id)
	}

	item := items[0]
	_, country := parseMany2One(item.Country)
	return &domain.Vendor{
		ID:      fmt.Sprintf("%d", item.ID),
		Name:    fmtStr(item.Name),
		Email:   fmtStr(item.Email),
		Phone:   fmtStr(item.Phone),
		Street:  fmtStr(item.Street),
		City:    fmtStr(item.City),
		Country: country,
		Balance: item.TotalDue,
	}, nil
}

func (o *OdooAdapter) GetPurchaseOrders(ctx context.Context, filter domain.PurchaseOrderFilter) ([]domain.PurchaseOrder, error) {
	domainFilter := []interface{}{}

	if filter.VendorID != "" {
		if vid, err := strconv.Atoi(filter.VendorID); err == nil {
			domainFilter = append(domainFilter, []interface{}{"partner_id", "=", vid})
		}
	}
	if filter.State != "" {
		domainFilter = append(domainFilter, []interface{}{"state", "=", filter.State})
	}
	if filter.Search != "" {
		domainFilter = append(domainFilter, []interface{}{"name", "ilike", filter.Search})
	}
	domainFilter = appendExtraDomain(domainFilter, filter.Domain)

	limit := 100
	if filter.Limit > 0 {
		limit = filter.Limit
	}

	kwargs := map[string]interface{}{
		"fields": []string{"id", "name", "partner_id", "date_order", "amount_untaxed", "amount_tax", "amount_total", "state", "order_line"},
		"limit":  limit,
		"offset": filter.Offset,
	}

	resRaw, err := o.executeKW(ctx, "purchase.order", "search_read", []interface{}{domainFilter}, kwargs)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch purchase orders from Odoo: %w", err)
	}

	var items []struct {
		ID            int     `json:"id"`
		Name          string  `json:"name"`
		PartnerID     any     `json:"partner_id"`
		DateOrder     string  `json:"date_order"`
		AmountUntaxed float64 `json:"amount_untaxed"`
		AmountTax     float64 `json:"amount_tax"`
		AmountTotal   float64 `json:"amount_total"`
		State         string  `json:"state"`
		OrderLine     []int   `json:"order_line"`
	}
	if err := json.Unmarshal(resRaw, &items); err != nil {
		return nil, fmt.Errorf("failed to parse Odoo purchase orders: %w", err)
	}

	orders := make([]domain.PurchaseOrder, 0, len(items))
	for _, item := range items {
		vid, vname := parseMany2One(item.PartnerID)
		dt, _ := time.Parse("2006-01-02 15:04:05", fmtStr(item.DateOrder))
		orders = append(orders, domain.PurchaseOrder{
			ID:            fmt.Sprintf("%d", item.ID),
			OrderNo:       fmtStr(item.Name),
			VendorID:      vid,
			VendorName:    vname,
			DateOrder:     dt,
			AmountUntaxed: item.AmountUntaxed,
			AmountTax:     item.AmountTax,
			AmountTotal:   item.AmountTotal,
			State:         fmtStr(item.State),
		})
	}

	return orders, nil
}

func (o *OdooAdapter) GetPurchaseOrderByID(ctx context.Context, id string) (*domain.PurchaseOrder, error) {
	idInt, err := strconv.Atoi(id)
	if err != nil {
		return nil, fmt.Errorf("invalid purchase order id format: %s", id)
	}

	domainFilter := []interface{}{
		[]interface{}{"id", "=", idInt},
	}
	kwargs := map[string]interface{}{
		"fields": []string{"id", "name", "partner_id", "date_order", "amount_untaxed", "amount_tax", "amount_total", "state", "order_line", "notes"},
		"limit":  1,
	}

	resRaw, err := o.executeKW(ctx, "purchase.order", "search_read", []interface{}{domainFilter}, kwargs)
	if err != nil {
		return nil, err
	}

	var items []struct {
		ID            int         `json:"id"`
		Name          string      `json:"name"`
		PartnerID     any         `json:"partner_id"`
		DateOrder     string      `json:"date_order"`
		AmountUntaxed float64     `json:"amount_untaxed"`
		AmountTax     float64     `json:"amount_tax"`
		AmountTotal   float64     `json:"amount_total"`
		State         string      `json:"state"`
		OrderLine     []int       `json:"order_line"`
		Notes         interface{} `json:"notes"`
	}
	if err := json.Unmarshal(resRaw, &items); err != nil || len(items) == 0 {
		return nil, fmt.Errorf("purchase order not found with ID %s", id)
	}

	item := items[0]
	vid, vname := parseMany2One(item.PartnerID)
	dt, _ := time.Parse("2006-01-02 15:04:05", fmtStr(item.DateOrder))

	lines := make([]domain.PurchaseOrderLine, 0)
	if len(item.OrderLine) > 0 {
		linesRaw, err := o.executeKW(ctx, "purchase.order.line", "search_read", []interface{}{
			[]interface{}{[]interface{}{"id", "in", item.OrderLine}},
		}, map[string]interface{}{
			"fields": []string{"id", "product_id", "name", "product_qty", "price_unit", "price_subtotal"},
		})
		if err == nil {
			var lineItems []struct {
				ID            int     `json:"id"`
				ProductID     string  `json:"product_id"`
				Name          string  `json:"name"`
				ProductQty    float64 `json:"product_qty"`
				PriceUnit     float64 `json:"price_unit"`
				PriceSubtotal float64 `json:"price_subtotal"`
			}
			if json.Unmarshal(linesRaw, &lineItems) == nil {
				for _, line := range lineItems {
					pid, pname := parseMany2One(line.ProductID)
					lines = append(lines, domain.PurchaseOrderLine{
						ID:            fmt.Sprintf("%d", line.ID),
						ProductID:     pid,
						ProductName:   pname,
						Quantity:      line.ProductQty,
						UnitPrice:     line.PriceUnit,
						PriceSubtotal: line.PriceSubtotal,
					})
				}
			}
		}
	}

	return &domain.PurchaseOrder{
		ID:            fmt.Sprintf("%d", item.ID),
		OrderNo:       fmtStr(item.Name),
		VendorID:      vid,
		VendorName:    vname,
		DateOrder:     dt,
		AmountUntaxed: item.AmountUntaxed,
		AmountTax:     item.AmountTax,
		AmountTotal:   item.AmountTotal,
		State:         fmtStr(item.State),
		Lines:         lines,
		Note:          fmtStr(item.Notes),
	}, nil
}

func (o *OdooAdapter) CreateRFQ(ctx context.Context, req domain.CreatePurchaseOrderRequest) (*domain.ActionResult, error) {
	vendorID, err := strconv.Atoi(req.VendorID)
	if err != nil {
		return nil, fmt.Errorf("invalid vendor ID %s: %w", req.VendorID, err)
	}

	vendor, err := o.GetVendorByID(ctx, req.VendorID)
	if err != nil {
		return nil, fmt.Errorf("vendor lookup failed for ID %s: %w", req.VendorID, err)
	}

	var orderLinesRPC []interface{}
	var calculatedUntaxed float64

	for _, l := range req.Lines {
		pid, err := strconv.Atoi(l.ProductID)
		if err != nil {
			return nil, fmt.Errorf("invalid product ID %s: %w", l.ProductID, err)
		}
		product, err := o.GetProductByID(ctx, l.ProductID)
		if err != nil {
			return nil, fmt.Errorf("product lookup failed for ID %s: %w", l.ProductID, err)
		}

		unitPrice := l.UnitPrice
		if unitPrice == 0 {
			unitPrice = product.StandardPrice
			if unitPrice == 0 {
				unitPrice = product.ListPrice
			}
		}
		calculatedUntaxed += unitPrice * l.Quantity

		lineData := map[string]interface{}{
			"product_id":  pid,
			"product_qty": l.Quantity,
			"price_unit":  unitPrice,
		}
		orderLinesRPC = append(orderLinesRPC, []interface{}{0, 0, lineData})
	}

	estimatedTax := calculatedUntaxed * 0.16
	totalEstimate := calculatedUntaxed + estimatedTax

	if req.DryRun {
		return &domain.ActionResult{
			Success: true,
			Message: fmt.Sprintf("DryRun simulation: RFQ for %s (Total: $%.2f) is ready to draft.", vendor.Name, totalEstimate),
			DryRun: &domain.DryRunResult{
				IsDryRun:             true,
				Operation:            "CREATE_RFQ",
				ImpactSummary:        fmt.Sprintf("Simulated creation of RFQ for %s with %d lines. Subtotal: $%.2f, Tax: $%.2f, Total: $%.2f", vendor.Name, len(req.Lines), calculatedUntaxed, estimatedTax, totalEstimate),
				RiskLevel:            domain.RiskLow,
				RequiresConfirmation: true,
				ConfirmationPrompt:   fmt.Sprintf("Do you want to draft RFQ for %s (Total: $%.2f)?", vendor.Name, totalEstimate),
				ConfirmationToken:    fmt.Sprintf("CONFIRM_CREATE_RFQ_%s_%d", vendor.ID, time.Now().Unix()),
			},
		}, nil
	}

	orderData := map[string]interface{}{
		"partner_id": vendorID,
		"order_line": orderLinesRPC,
		"notes":      req.Note,
	}

	resRaw, err := o.executeKW(ctx, "purchase.order", "create", []interface{}{orderData}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create RFQ in Odoo: %w", err)
	}

	var createdID int
	if err := json.Unmarshal(resRaw, &createdID); err != nil {
		return nil, fmt.Errorf("failed to parse created RFQ ID: %w", err)
	}

	createdOrder, err := o.GetPurchaseOrderByID(ctx, fmt.Sprintf("%d", createdID))
	if err != nil {
		return &domain.ActionResult{
			Success:      true,
			Message:      fmt.Sprintf("RFQ created successfully with ID %d", createdID),
			RecordID:     fmt.Sprintf("%d", createdID),
			RecordNumber: fmt.Sprintf("PO%d", createdID),
		}, nil
	}

	return &domain.ActionResult{
		Success:      true,
		Message:      fmt.Sprintf("RFQ %s drafted successfully for %s!", createdOrder.OrderNo, createdOrder.VendorName),
		RecordID:     createdOrder.ID,
		RecordNumber: createdOrder.OrderNo,
		Data:         createdOrder,
	}, nil
}

func (o *OdooAdapter) ConfirmPurchaseOrder(ctx context.Context, orderID string, dryRun bool) (*domain.ActionResult, error) {
	idInt, err := strconv.Atoi(orderID)
	if err != nil {
		return nil, fmt.Errorf("invalid purchase order id %s", orderID)
	}

	order, err := o.GetPurchaseOrderByID(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("cannot confirm PO: order ID %s not found", orderID)
	}

	if order.State != "draft" && order.State != "sent" && order.State != "to approve" && order.State != "rfq" {
		return nil, fmt.Errorf("purchase order %s is in state '%s' and cannot be confirmed (must be draft/RFQ)", order.OrderNo, order.State)
	}

	if dryRun {
		return &domain.ActionResult{
			Success: true,
			Message: fmt.Sprintf("DryRun simulation: PO %s (%s) is ready to be confirmed.", order.OrderNo, order.VendorName),
			DryRun: &domain.DryRunResult{
				IsDryRun:             true,
				Operation:            "CONFIRM_PURCHASE_ORDER",
				ImpactSummary:        fmt.Sprintf("Confirming PO %s will change state from '%s' to 'purchase' (Confirmed), committing $%.2f spend with %s.", order.OrderNo, order.State, order.AmountTotal, order.VendorName),
				RiskLevel:            domain.RiskMedium,
				RequiresConfirmation: true,
				ConfirmationPrompt:   fmt.Sprintf("Confirming PO %s commits $%.2f with %s. Do you want to proceed?", order.OrderNo, order.AmountTotal, order.VendorName),
				ConfirmationToken:    fmt.Sprintf("TOKEN_CONFIRM_PO_%s_%d", orderID, time.Now().Unix()),
			},
		}, nil
	}

	// button_confirm is stable across Odoo 17/18/19 (see version.go).
	if _, err := o.executeKW(ctx, "purchase.order", "button_confirm", []interface{}{[]int{idInt}}, nil); err != nil {
		return nil, fmt.Errorf("failed to confirm purchase order %s in Odoo: %w", orderID, err)
	}

	return &domain.ActionResult{
		Success:      true,
		Message:      fmt.Sprintf("Purchase Order %s confirmed successfully!", order.OrderNo),
		RecordID:     orderID,
		RecordNumber: order.OrderNo,
	}, nil
}

func (o *OdooAdapter) CancelPurchaseOrder(ctx context.Context, orderID string, dryRun bool) (*domain.ActionResult, error) {
	idInt, err := strconv.Atoi(orderID)
	if err != nil {
		return nil, fmt.Errorf("invalid purchase order id %s", orderID)
	}

	order, err := o.GetPurchaseOrderByID(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("purchase order ID %s not found", orderID)
	}

	if order.State != "draft" && order.State != "sent" && order.State != "to approve" && order.State != "rfq" {
		return nil, fmt.Errorf("purchase order %s is in state '%s' and cannot be cancelled in v1 (draft/RFQ only)", order.OrderNo, order.State)
	}

	if dryRun {
		return &domain.ActionResult{
			Success: true,
			Message: fmt.Sprintf("DryRun simulation: RFQ %s cancellation preview.", order.OrderNo),
			DryRun: &domain.DryRunResult{
				IsDryRun:             true,
				Operation:            "CANCEL_PURCHASE_ORDER",
				ImpactSummary:        fmt.Sprintf("Cancelling RFQ %s will void the draft request to %s.", order.OrderNo, order.VendorName),
				RiskLevel:            domain.RiskMedium,
				RequiresConfirmation: true,
				ConfirmationPrompt:   fmt.Sprintf("CAUTION: Cancel RFQ %s to %s? This voids the draft.", order.OrderNo, order.VendorName),
				ConfirmationToken:    fmt.Sprintf("TOKEN_CANCEL_PO_%s_%d", orderID, time.Now().Unix()),
			},
		}, nil
	}

	if _, err := o.executeKW(ctx, "purchase.order", "button_cancel", []interface{}{[]int{idInt}}, nil); err != nil {
		return nil, fmt.Errorf("failed to cancel purchase order %s in Odoo: %w", orderID, err)
	}

	return &domain.ActionResult{
		Success:      true,
		Message:      fmt.Sprintf("RFQ %s has been cancelled.", order.OrderNo),
		RecordID:     orderID,
		RecordNumber: order.OrderNo,
	}, nil
}

func (o *OdooAdapter) ReceivePurchaseOrder(ctx context.Context, orderID string, dryRun bool) (*domain.ActionResult, error) {
	order, err := o.GetPurchaseOrderByID(ctx, orderID)
	if err != nil {
		return nil, err
	}

	if order.State != "purchase" {
		return nil, fmt.Errorf("purchase order %s is in state '%s' and cannot be received (must be confirmed 'purchase')", order.OrderNo, order.State)
	}

	if dryRun {
		return &domain.ActionResult{
			Success: true,
			Message: fmt.Sprintf("DryRun simulation: Full receipt preview for PO %s.", order.OrderNo),
			DryRun: &domain.DryRunResult{
				IsDryRun:             true,
				Operation:            "RECEIVE_PURCHASE_ORDER",
				ImpactSummary:        fmt.Sprintf("Receiving PO %s will validate the full inbound picking and increase inventory stock (v1: full receipt only, no partials/backorders).", order.OrderNo),
				RiskLevel:            domain.RiskHigh,
				RequiresConfirmation: true,
				ConfirmationPrompt:   fmt.Sprintf("Receive ALL lines of PO %s from %s into stock? (Full receipt only.)", order.OrderNo, order.VendorName),
				ConfirmationToken:    fmt.Sprintf("TOKEN_RECEIPT_%s_%d", orderID, time.Now().Unix()),
			},
		}, nil
	}

	idInt, _ := strconv.Atoi(orderID)

	// Find open inbound pickings linked to this PO.
	domainFilter := []interface{}{
		[]interface{}{"purchase_id", "=", idInt},
		[]interface{}{"state", "not in", []string{"done", "cancel"}},
	}
	resPickings, err := o.executeKW(ctx, "stock.picking", "search", []interface{}{domainFilter}, nil)
	if err == nil {
		var pickingIDs []int
		if json.Unmarshal(resPickings, &pickingIDs) == nil && len(pickingIDs) > 0 {
			_, _ = o.executeKW(ctx, "stock.picking", "action_assign", []interface{}{pickingIDs}, nil)
			_, _ = o.executeKW(ctx, "stock.picking", "action_set_quantities_to_reservation", []interface{}{pickingIDs}, nil)
			if _, validateErr := o.executeKW(ctx, "stock.picking", "button_validate", []interface{}{pickingIDs}, nil); validateErr != nil {
				return nil, fmt.Errorf("failed to validate receipt picking: %w", validateErr)
			}
		}
	}

	return &domain.ActionResult{
		Success:      true,
		Message:      fmt.Sprintf("Receipt for PO %s processed (full receipt)!", order.OrderNo),
		RecordID:     orderID,
		RecordNumber: order.OrderNo,
	}, nil
}

func (o *OdooAdapter) GetVendorBills(ctx context.Context, filter domain.VendorBillFilter) ([]domain.VendorBill, error) {
	domainFilter := []interface{}{
		[]interface{}{"move_type", "=", "in_invoice"},
	}
	if filter.VendorID != "" {
		if vid, err := strconv.Atoi(filter.VendorID); err == nil {
			domainFilter = append(domainFilter, []interface{}{"partner_id", "=", vid})
		}
	}
	if filter.Status != "" {
		// Map canonical status to Odoo account.move state/payment_state loosely.
		if strings.EqualFold(filter.Status, "posted") || strings.EqualFold(filter.Status, "paid") {
			domainFilter = append(domainFilter, []interface{}{"state", "=", "posted"})
		} else {
			domainFilter = append(domainFilter, []interface{}{"state", "=", "draft"})
		}
	}

	limit := 100
	if filter.Limit > 0 {
		limit = filter.Limit
	}

	kwargs := map[string]interface{}{
		"fields": []string{"id", "name", "partner_id", "invoice_origin", "move_type", "amount_total", "amount_residual", "state", "invoice_date_due"},
		"limit":  limit,
		"offset": filter.Offset,
	}

	resRaw, err := o.executeKW(ctx, "account.move", "search_read", []interface{}{domainFilter}, kwargs)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch vendor bills from Odoo: %w", err)
	}

	var items []struct {
		ID             int         `json:"id"`
		Name           string      `json:"name"`
		PartnerID      any         `json:"partner_id"`
		InvoiceOrigin  interface{} `json:"invoice_origin"`
		MoveType       string      `json:"move_type"`
		AmountTotal    float64     `json:"amount_total"`
		AmountResidual float64     `json:"amount_residual"`
		State          string      `json:"state"`
		DueDate        string      `json:"invoice_date_due"`
	}
	if err := json.Unmarshal(resRaw, &items); err != nil {
		return nil, fmt.Errorf("failed to parse Odoo vendor bills: %w", err)
	}

	bills := make([]domain.VendorBill, 0, len(items))
	for _, item := range items {
		vid, vname := parseMany2One(item.PartnerID)
		dt, _ := time.Parse("2006-01-02", item.DueDate)
		status := item.State
		if strings.EqualFold(item.State, "posted") && item.AmountResidual == 0 {
			status = "paid"
		}
		bills = append(bills, domain.VendorBill{
			ID:          fmt.Sprintf("%d", item.ID),
			BillNo:      item.Name,
			VendorID:    vid,
			VendorName:  vname,
			OrderID:     fmtStr(item.InvoiceOrigin),
			MoveType:    item.MoveType,
			AmountTotal: item.AmountTotal,
			AmountDue:   item.AmountResidual,
			Status:      status,
			DueDate:     dt,
		})
	}

	return bills, nil
}

func (o *OdooAdapter) CreateVendorBill(ctx context.Context, orderID string, dryRun bool) (*domain.ActionResult, error) {
	idInt, err := strconv.Atoi(orderID)
	if err != nil {
		return nil, fmt.Errorf("invalid purchase order id %s", orderID)
	}

	order, err := o.GetPurchaseOrderByID(ctx, orderID)
	if err != nil {
		return nil, err
	}

	if order.State != "purchase" && order.State != "done" {
		return nil, fmt.Errorf("purchase order %s is in state '%s'; bill only after confirm/receive", order.OrderNo, order.State)
	}

	if dryRun {
		return &domain.ActionResult{
			Success: true,
			Message: fmt.Sprintf("DryRun simulation: Vendor bill preview for PO %s ($%.2f).", order.OrderNo, order.AmountTotal),
			DryRun: &domain.DryRunResult{
				IsDryRun:             true,
				Operation:            "CREATE_VENDOR_BILL",
				ImpactSummary:        fmt.Sprintf("Drafting vendor bill from PO %s (Total: $%.2f, auto-copied lines, no edits in v1).", order.OrderNo, order.AmountTotal),
				RiskLevel:            domain.RiskMedium,
				RequiresConfirmation: true,
				ConfirmationPrompt:   fmt.Sprintf("Draft vendor bill for $%.2f from PO %s?", order.AmountTotal, order.OrderNo),
				ConfirmationToken:    fmt.Sprintf("TOKEN_CREATE_BILL_%s_%d", orderID, time.Now().Unix()),
			},
		}, nil
	}

	// action_create_invoice generates the draft vendor bill from the PO (stable 17/18/19).
	if _, err := o.executeKW(ctx, "purchase.order", "action_create_invoice", []interface{}{[]int{idInt}}, nil); err != nil {
		return nil, fmt.Errorf("failed to create vendor bill from PO %s in Odoo: %w", orderID, err)
	}

	return &domain.ActionResult{
		Success:      true,
		Message:      fmt.Sprintf("Vendor bill created successfully from PO %s!", order.OrderNo),
		RecordID:     orderID,
		RecordNumber: order.OrderNo,
	}, nil
}

func (o *OdooAdapter) PostVendorBill(ctx context.Context, billID string, dryRun bool) (*domain.ActionResult, error) {
	idInt, err := strconv.Atoi(billID)
	if err != nil {
		return nil, fmt.Errorf("invalid vendor bill id %s", billID)
	}

	if dryRun {
		return &domain.ActionResult{
			Success: true,
			Message: fmt.Sprintf("DryRun simulation: Post vendor bill preview for %s.", billID),
			DryRun: &domain.DryRunResult{
				IsDryRun:             true,
				Operation:            "POST_VENDOR_BILL",
				ImpactSummary:        fmt.Sprintf("Posting vendor bill %s will finalize accounting entry (post-only in v1, no payment execution).", billID),
				RiskLevel:            domain.RiskHigh,
				RequiresConfirmation: true,
				ConfirmationPrompt:   fmt.Sprintf("Post vendor bill %s? (Posts journal only, no payment.)", billID),
				ConfirmationToken:    fmt.Sprintf("TOKEN_POST_BILL_%s_%d", billID, time.Now().Unix()),
			},
		}, nil
	}

	if _, err := o.executeKW(ctx, "account.move", "action_post", []interface{}{[]int{idInt}}, nil); err != nil {
		return nil, fmt.Errorf("failed to post vendor bill %s in Odoo: %w", billID, err)
	}

	return &domain.ActionResult{
		Success:      true,
		Message:      fmt.Sprintf("Vendor bill %s posted successfully!", billID),
		RecordID:     billID,
		RecordNumber: billID,
	}, nil
}
