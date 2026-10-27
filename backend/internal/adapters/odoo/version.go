package odoo

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

// version.go — Odoo 17/18/19 compatibility shims.
//
// Strategy (locked Q13): single OdooAdapter for all supported versions. Version-specific
// method differences are isolated here behind small capability switches; the canonical
// domain models and NL tools stay version-blind.
//
// Known-stable across 17/18/19 (used directly in purchase.go / odoo_adapter.go):
//   - purchase.order: button_confirm, button_cancel, action_create_invoice, create, search_read
//   - purchase.order.line: search_read (product_id, product_qty, price_unit, price_subtotal)
//   - stock.picking: search, action_assign, action_set_quantities_to_reservation, button_validate
//   - account.move: search_read, action_post (in_invoice vendor bills)
//   - res.partner: search_read (supplier_rank >= 1 marks vendors)
//   - sale.order: action_confirm, action_cancel; sale.advance.payment.inv: create + create_invoices
//
// If a future version renames any of these, add a method() switch here and call it
// from the adapter instead of hardcoding the string.

type odooVersion struct {
	Major int
	Full  string
}

var (
	versionCache   *odooVersion
	versionCacheMu sync.Mutex
)

// DetectVersion probes the Odoo server version via the common service and caches it.
// Returns cached value on subsequent calls. Falls back to major 19 (current live)
// when the server is unreachable so dry-run/mock flows keep working.
func (o *OdooAdapter) DetectVersion(ctx context.Context) *odooVersion {
	versionCacheMu.Lock()
	defer versionCacheMu.Unlock()

	if versionCache != nil {
		return versionCache
	}

	versionCache = &odooVersion{Major: 19, Full: "19.0 (assumed)"}

	payload := jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  "call",
		Params: map[string]interface{}{
			"service": "common",
			"method":  "version",
			"args":    []interface{}{},
		},
		ID: 1,
	}

	raw, err := o.sendRPC(ctx, payload)
	if err != nil {
		return versionCache
	}

	var info struct {
		ServerVersion string `json:"server_version"`
	}
	if err := json.Unmarshal(raw, &info); err != nil {
		return versionCache
	}

	major := parseMajorVersion(info.ServerVersion)
	if major >= 15 && major <= 20 {
		versionCache = &odooVersion{Major: major, Full: info.ServerVersion}
	}

	return versionCache
}

// parseMajorVersion extracts the leading major number from strings like "19.0", "18.0+e".
func parseMajorVersion(s string) int {
	s = strings.TrimSpace(s)
	if idx := strings.IndexAny(s, ".+- "); idx > 0 {
		s = s[:idx]
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

// PurchaseConfirmMethod returns the confirm method for purchase.order.
// Stable as button_confirm on 17/18/19; centralized here for future-proofing.
func (o *OdooAdapter) PurchaseConfirmMethod(ctx context.Context) string {
	v := o.DetectVersion(ctx)
	switch v.Major {
	case 17, 18, 19:
		return "button_confirm"
	default:
		return fmt.Sprintf("button_confirm /* untested on Odoo %d, assuming 19 semantics */", v.Major)
	}
}

// PurchaseCancelMethod returns the cancel method for purchase.order.
func (o *OdooAdapter) PurchaseCancelMethod(ctx context.Context) string {
	return "button_cancel" // stable 17/18/19
}

// PickingValidateMethod returns the validate method for stock.picking.
func (o *OdooAdapter) PickingValidateMethod(ctx context.Context) string {
	return "button_validate" // stable 17/18/19
}

// BillCreateMethod returns the PO-to-bill method for purchase.order.
func (o *OdooAdapter) BillCreateMethod(ctx context.Context) string {
	return "action_create_invoice" // stable 17/18/19
}
