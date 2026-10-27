package domain

import "context"

// Count entities accepted by RecordCounter.
const (
	CountEntityCustomers = "customers"
	CountEntityProducts  = "products"
	CountEntityOrders    = "orders"
	CountEntityInvoices  = "invoices"
	CountEntityVendors   = "vendors"
)

// CountFilter is the narrow filter surface for aggregate questions ("how many").
// Adapters translate it to an exact server-side count instead of fetching rows.
type CountFilter struct {
	CustomerID   string `json:"customer_id,omitempty"`
	Search       string `json:"search,omitempty"`
	State        string `json:"state,omitempty"`
	PaymentState string `json:"payment_state,omitempty"`
	Overdue      bool   `json:"overdue,omitempty"`
}

// RecordCounter is implemented by adapters that can answer counts exactly.
// Handlers fall back to a bounded fetch when it is absent.
type RecordCounter interface {
	CountRecords(ctx context.Context, entity string, filter CountFilter) (int, error)
}
