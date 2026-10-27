package domain

import "time"

// Customer represents the Canonical Customer Model.
type Customer struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Email       string  `json:"email"`
	Phone       string  `json:"phone"`
	CreditLimit float64 `json:"credit_limit"`
	Balance     float64 `json:"balance"`
	Street      string  `json:"street,omitempty"`
	City        string  `json:"city,omitempty"`
	Country     string  `json:"country,omitempty"`
}

// CustomerFilter defines search criteria for customers.
type CustomerFilter struct {
	Search string `json:"search,omitempty"`
	Email  string `json:"email,omitempty"`
	Limit  int    `json:"limit,omitempty"`
	Offset int    `json:"offset,omitempty"`
	// Domain carries an allowlisted, AI-compiled Odoo domain (e.g.
	// [["sale_order_count", ">", 1]]). Validated upstream; never raw user text.
	Domain []any `json:"domain,omitempty"`
}

// Product represents the Canonical Product Model.
type Product struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	SKU           string  `json:"sku"`
	ListPrice     float64 `json:"list_price"`
	StandardPrice float64 `json:"standard_price,omitempty"`
	QtyAvailable  float64 `json:"qty_available"`
	UnitOfMeasure string  `json:"unit_of_measure,omitempty"`
	Category      string  `json:"category,omitempty"`
}

// ProductFilter defines search criteria for products.
type ProductFilter struct {
	Search string `json:"search,omitempty"`
	SKU    string `json:"sku,omitempty"`
	Limit  int    `json:"limit,omitempty"`
	Offset int    `json:"offset,omitempty"`
	Domain []any  `json:"domain,omitempty"`
}

// StockTransferRequest represents payload for moving inventory stock.
type StockTransferRequest struct {
	ProductSKU string  `json:"product_sku"`
	Quantity   float64 `json:"quantity"`
	FromLocID  string  `json:"from_loc_id"`
	ToLocID    string  `json:"to_loc_id"`
	DryRun     bool    `json:"dry_run"`
}

// SaleOrderLine represents an individual item in a Sales Order or Quotation.
type SaleOrderLine struct {
	ID            string  `json:"id,omitempty"`
	ProductID     string  `json:"product_id"`
	ProductName   string  `json:"product_name,omitempty"`
	Quantity      float64 `json:"quantity"`
	UnitPrice     float64 `json:"unit_price"`
	PriceSubtotal float64 `json:"price_subtotal"`
	TaxAmount     float64 `json:"tax_amount,omitempty"`
}

// SaleOrder represents Canonical Sale Order and Quotation Model.
type SaleOrder struct {
	ID            string          `json:"id"`
	OrderNo       string          `json:"order_no"`
	CustomerID    string          `json:"customer_id"`
	CustomerName  string          `json:"customer_name,omitempty"`
	DateOrder     time.Time       `json:"date_order"`
	AmountUntaxed float64         `json:"amount_untaxed"`
	AmountTax     float64         `json:"amount_tax"`
	AmountTotal   float64         `json:"amount_total"`
	State         string          `json:"state"` // "draft" (Quotation), "sent", "sale" (Confirmed), "done" (Delivered), "cancel"
	Lines         []SaleOrderLine `json:"lines"`
	Note          string          `json:"note,omitempty"`
	DeliveryState string          `json:"delivery_state,omitempty"` // "pending", "partially_delivered", "fully_delivered"
	InvoiceState  string          `json:"invoice_state,omitempty"`  // "no", "to_invoice", "invoiced"
}

// SaleOrderFilter defines search criteria for sale orders.
type SaleOrderFilter struct {
	CustomerID string `json:"customer_id,omitempty"`
	State      string `json:"state,omitempty"`
	Search     string `json:"search,omitempty"`
	Limit      int    `json:"limit,omitempty"`
	Offset     int    `json:"offset,omitempty"`
	Domain     []any  `json:"domain,omitempty"`
}

// CreateSaleOrderLineRequest represents line item payload for order/quotation creation.
type CreateSaleOrderLineRequest struct {
	ProductID string  `json:"product_id"`
	Quantity  float64 `json:"quantity"`
	UnitPrice float64 `json:"unit_price,omitempty"`
}

// CreateSaleOrderRequest represents payload for creating a new sales order or quotation.
type CreateSaleOrderRequest struct {
	CustomerID  string                       `json:"customer_id"`
	Lines       []CreateSaleOrderLineRequest `json:"lines"`
	Note        string                       `json:"note,omitempty"`
	IsQuotation bool                         `json:"is_quotation"`
	DryRun      bool                         `json:"dry_run"`
}

// DeliveryLine represents items to be delivered.
type DeliveryLine struct {
	ProductID    string  `json:"product_id"`
	ProductName  string  `json:"product_name"`
	QtyOrdered   float64 `json:"qty_ordered"`
	QtyDelivered float64 `json:"qty_delivered"`
}

// DeliveryOrder represents Canonical Delivery / Stock Picking.
type DeliveryOrder struct {
	ID            string         `json:"id"`
	DeliveryNo    string         `json:"delivery_no"`
	OrderID       string         `json:"order_id"`
	OrderNo       string         `json:"order_no"`
	CustomerID    string         `json:"customer_id"`
	CustomerName  string         `json:"customer_name"`
	State         string         `json:"state"` // "draft", "waiting", "ready", "done", "cancelled"
	Lines         []DeliveryLine `json:"lines"`
	ScheduledDate time.Time      `json:"scheduled_date"`
}

// Invoice represents Canonical Invoice Model.
type Invoice struct {
	ID           string    `json:"id"`
	InvoiceNo    string    `json:"invoice_no"`
	CustomerID   string    `json:"customer_id"`
	CustomerName string    `json:"customer_name,omitempty"`
	OrderID      string    `json:"order_id,omitempty"`
	MoveType     string    `json:"move_type"` // "out_invoice" (Standard Invoice), "out_refund" (Credit Note)
	AmountTotal  float64   `json:"amount_total"`
	AmountDue    float64   `json:"amount_due"`
	Status       string    `json:"status"`        // "draft", "posted", "paid", "cancel"
	PaymentState string    `json:"payment_state"` // "not_paid", "partial", "paid", "in_payment"
	DueDate      time.Time `json:"due_date"`
	ETIMSStatus  string    `json:"etims_status,omitempty"` // "pending", "fiscalized", "failed"
	CUSerialNo   string    `json:"cu_serial_no,omitempty"`
	IRN          string    `json:"irn,omitempty"`
	QRCodeURL    string    `json:"qr_code_url,omitempty"`
}

// InvoiceFilter defines search criteria for invoices and credit notes.
type InvoiceFilter struct {
	CustomerID   string  `json:"customer_id,omitempty"`
	OrderID      string  `json:"order_id,omitempty"`
	MoveType     string  `json:"move_type,omitempty"`
	Status       string  `json:"status,omitempty"`
	PaymentState string  `json:"payment_state,omitempty"`
	Overdue      bool    `json:"overdue,omitempty"`
	MinAmount    float64 `json:"min_amount,omitempty"`
	Limit        int     `json:"limit,omitempty"`
	Offset       int     `json:"offset,omitempty"`
	Domain       []any   `json:"domain,omitempty"`
}

// CreditNoteRequest payload for issuing credit notes.
type CreditNoteRequest struct {
	InvoiceID string  `json:"invoice_id"`
	Reason    string  `json:"reason"`
	Amount    float64 `json:"amount,omitempty"` // Partial or full refund amount
	DryRun    bool    `json:"dry_run"`
}

// ETIMSSubmissionResult response from eTIMS / KRA fiscalization.
type ETIMSSubmissionResult struct {
	InvoiceID        string    `json:"invoice_id"`
	InvoiceNo        string    `json:"invoice_no"`
	FiscalStatus     string    `json:"fiscal_status"` // "SUCCESS", "FAILED"
	CUSerialNo       string    `json:"cu_serial_no"`
	IRN              string    `json:"irn"` // Invoice Registration Number
	QRCodeURL        string    `json:"qr_code_url"`
	FiscalizedAmount float64   `json:"fiscalized_amount"`
	TaxAmount        float64   `json:"tax_amount"`
	Timestamp        time.Time `json:"timestamp"`
	Message          string    `json:"message"`
}

// PrintDocumentRequest payload for rendering documents.
type PrintDocumentRequest struct {
	DocumentType string `json:"document_type"` // "quotation", "sale_order", "invoice", "delivery_note", "credit_note"
	DocumentID   string `json:"document_id"`
	Format       string `json:"format"` // "pdf", "html"
}

// PrintDocumentResult response with rendered document payload.
type PrintDocumentResult struct {
	DocumentType string `json:"document_type"`
	DocumentID   string `json:"document_id"`
	DocumentNo   string `json:"document_no"`
	Format       string `json:"format"`
	DownloadURL  string `json:"download_url"`
	PDFBase64    string `json:"pdf_base64,omitempty"`
}

// Vendor represents the Canonical Vendor / Supplier Model (portable, not Odoo-specific).
type Vendor struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Email   string  `json:"email"`
	Phone   string  `json:"phone"`
	Street  string  `json:"street,omitempty"`
	City    string  `json:"city,omitempty"`
	Country string  `json:"country,omitempty"`
	Balance float64 `json:"balance"`
}

// VendorFilter defines search criteria for vendors.
type VendorFilter struct {
	Search string `json:"search,omitempty"`
	Email  string `json:"email,omitempty"`
	Limit  int    `json:"limit,omitempty"`
	Offset int    `json:"offset,omitempty"`
	Domain []any  `json:"domain,omitempty"`
}

// PurchaseOrderLine represents an individual item in an RFQ / Purchase Order.
type PurchaseOrderLine struct {
	ID            string  `json:"id,omitempty"`
	ProductID     string  `json:"product_id"`
	ProductName   string  `json:"product_name,omitempty"`
	Quantity      float64 `json:"quantity"`
	UnitPrice     float64 `json:"unit_price"`
	PriceSubtotal float64 `json:"price_subtotal"`
}

// PurchaseOrder represents Canonical RFQ / Purchase Order Model.
type PurchaseOrder struct {
	ID            string              `json:"id"`
	OrderNo       string              `json:"order_no"`
	VendorID      string              `json:"vendor_id"`
	VendorName    string              `json:"vendor_name,omitempty"`
	DateOrder     time.Time           `json:"date_order"`
	AmountUntaxed float64             `json:"amount_untaxed"`
	AmountTax     float64             `json:"amount_tax"`
	AmountTotal   float64             `json:"amount_total"`
	State         string              `json:"state"` // "draft" (RFQ), "sent", "purchase" (Confirmed), "done" (Received), "cancel"
	Lines         []PurchaseOrderLine `json:"lines"`
	Note          string              `json:"note,omitempty"`
	ReceiptState  string              `json:"receipt_state,omitempty"` // "pending", "partially_received", "fully_received"
	BillingState  string              `json:"billing_state,omitempty"` // "no", "to_bill", "billed"
}

// PurchaseOrderFilter defines search criteria for purchase orders.
type PurchaseOrderFilter struct {
	VendorID string `json:"vendor_id,omitempty"`
	State    string `json:"state,omitempty"`
	Search   string `json:"search,omitempty"`
	Limit    int    `json:"limit,omitempty"`
	Offset   int    `json:"offset,omitempty"`
	Domain   []any  `json:"domain,omitempty"`
}

// CreatePurchaseOrderLineRequest represents line item payload for RFQ/PO creation.
type CreatePurchaseOrderLineRequest struct {
	ProductID string  `json:"product_id"`
	Quantity  float64 `json:"quantity"`
	UnitPrice float64 `json:"unit_price,omitempty"`
}

// CreatePurchaseOrderRequest represents payload for creating a new RFQ / purchase order.
type CreatePurchaseOrderRequest struct {
	VendorID string                           `json:"vendor_id"`
	Lines    []CreatePurchaseOrderLineRequest `json:"lines"`
	Note     string                           `json:"note,omitempty"`
	DryRun   bool                             `json:"dry_run"`
}

// Receipt represents Canonical Goods Receipt / Stock Picking (inbound).
type Receipt struct {
	ID            string    `json:"id"`
	ReceiptNo     string    `json:"receipt_no"`
	OrderID       string    `json:"order_id"`
	OrderNo       string    `json:"order_no"`
	VendorID      string    `json:"vendor_id"`
	VendorName    string    `json:"vendor_name"`
	State         string    `json:"state"` // "draft", "waiting", "ready", "done", "cancelled"
	ScheduledDate time.Time `json:"scheduled_date"`
}

// VendorBill represents Canonical Vendor Bill Model.
type VendorBill struct {
	ID          string    `json:"id"`
	BillNo      string    `json:"bill_no"`
	VendorID    string    `json:"vendor_id"`
	VendorName  string    `json:"vendor_name,omitempty"`
	OrderID     string    `json:"order_id,omitempty"`
	MoveType    string    `json:"move_type"` // "in_invoice" (Vendor Bill), "in_refund"
	AmountTotal float64   `json:"amount_total"`
	AmountDue   float64   `json:"amount_due"`
	Status      string    `json:"status"` // "draft", "posted", "paid", "cancel"
	DueDate     time.Time `json:"due_date"`
}

// VendorBillFilter defines search criteria for vendor bills.
type VendorBillFilter struct {
	VendorID  string  `json:"vendor_id,omitempty"`
	MoveType  string  `json:"move_type,omitempty"`
	Status    string  `json:"status,omitempty"`
	MinAmount float64 `json:"min_amount,omitempty"`
	Limit     int     `json:"limit,omitempty"`
	Offset    int     `json:"offset,omitempty"`
}

// ValidationPolicy & Approval Details
type RiskLevel string

const (
	RiskLow    RiskLevel = "LOW"
	RiskMedium RiskLevel = "MEDIUM"
	RiskHigh   RiskLevel = "HIGH"
)

// DryRunResult represents preview calculations, impact analysis, and human approval prompts.
type DryRunResult struct {
	IsDryRun              bool       `json:"is_dry_run"`
	Operation             string     `json:"operation"`
	ImpactSummary         string     `json:"impact_summary"`
	RiskLevel             RiskLevel  `json:"risk_level"`
	RequiresConfirmation  bool       `json:"requires_confirmation"`
	ConfirmationPrompt    string     `json:"confirmation_prompt"` // Explicit user query
	ConfirmationToken     string     `json:"confirmation_token,omitempty"`
	SimulatedOrderDetails *SaleOrder `json:"simulated_order_details,omitempty"`
	SimulatedInvoice      *Invoice   `json:"simulated_invoice,omitempty"`
}

// ActionResult represents execution response from ERP mutations.
type ActionResult struct {
	Success      bool          `json:"success"`
	Message      string        `json:"message"`
	RecordID     string        `json:"record_id,omitempty"`
	RecordNumber string        `json:"record_number,omitempty"`
	DryRun       *DryRunResult `json:"dry_run,omitempty"`
	Data         any           `json:"data,omitempty"`
}
