package domain

import "context"

// ERPConnector defines the Universal ERP Interface across all supported modules.
type ERPConnector interface {
	// Customer Operations
	GetCustomers(ctx context.Context, filter CustomerFilter) ([]Customer, error)
	GetCustomerByID(ctx context.Context, id string) (*Customer, error)

	// Product Operations
	GetProducts(ctx context.Context, filter ProductFilter) ([]Product, error)
	GetProductByID(ctx context.Context, id string) (*Product, error)

	// Quotation & Sales Order Operations
	GetSaleOrders(ctx context.Context, filter SaleOrderFilter) ([]SaleOrder, error)
	GetSaleOrderByID(ctx context.Context, id string) (*SaleOrder, error)
	CreateQuotation(ctx context.Context, req CreateSaleOrderRequest) (*ActionResult, error)
	SendQuotation(ctx context.Context, quotationID string) (*ActionResult, error)
	ConvertQuotationToOrder(ctx context.Context, quotationID string, dryRun bool) (*ActionResult, error)
	CreateSaleOrder(ctx context.Context, req CreateSaleOrderRequest) (*ActionResult, error)
	ConfirmSaleOrder(ctx context.Context, orderID string, dryRun bool) (*ActionResult, error)
	CancelSaleOrder(ctx context.Context, orderID string, dryRun bool) (*ActionResult, error)
	DeliverSaleOrder(ctx context.Context, orderID string, dryRun bool) (*ActionResult, error)

	// Invoice & Credit Note Operations
	GetInvoices(ctx context.Context, filter InvoiceFilter) ([]Invoice, error)
	GetOverdueInvoices(ctx context.Context, minAmount float64) ([]Invoice, error)
	CreateInvoice(ctx context.Context, orderID string, dryRun bool) (*ActionResult, error)
	PostInvoice(ctx context.Context, invoiceID string, dryRun bool) (*ActionResult, error)
	CreateCreditNote(ctx context.Context, req CreditNoteRequest) (*ActionResult, error)

	// eTIMS / KRA Fiscalization Compliance
	SubmitETIMS(ctx context.Context, invoiceID string, dryRun bool) (*ETIMSSubmissionResult, error)

	// Document Printing / PDF Generation
	PrintDocument(ctx context.Context, req PrintDocumentRequest) (*PrintDocumentResult, error)

	// Inventory Operations
	GetStockLevel(ctx context.Context, sku string, locationID string) (float64, error)
	TransferStock(ctx context.Context, req StockTransferRequest) (*ActionResult, error)

	// Vendor Operations (portable; ERP-specific mapping lives in adapters)
	GetVendors(ctx context.Context, filter VendorFilter) ([]Vendor, error)
	GetVendorByID(ctx context.Context, id string) (*Vendor, error)

	// RFQ & Purchase Order Operations
	GetPurchaseOrders(ctx context.Context, filter PurchaseOrderFilter) ([]PurchaseOrder, error)
	GetPurchaseOrderByID(ctx context.Context, id string) (*PurchaseOrder, error)
	CreateRFQ(ctx context.Context, req CreatePurchaseOrderRequest) (*ActionResult, error)
	ConfirmPurchaseOrder(ctx context.Context, orderID string, dryRun bool) (*ActionResult, error)
	CancelPurchaseOrder(ctx context.Context, orderID string, dryRun bool) (*ActionResult, error)
	ReceivePurchaseOrder(ctx context.Context, orderID string, dryRun bool) (*ActionResult, error)

	// Vendor Bill Operations
	GetVendorBills(ctx context.Context, filter VendorBillFilter) ([]VendorBill, error)
	CreateVendorBill(ctx context.Context, orderID string, dryRun bool) (*ActionResult, error)
	PostVendorBill(ctx context.Context, billID string, dryRun bool) (*ActionResult, error)
}
