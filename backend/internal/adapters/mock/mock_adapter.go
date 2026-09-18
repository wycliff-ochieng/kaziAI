package mock

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"erpagent-backend/internal/domain"
)

type MockERPAdapter struct {
	mu             sync.RWMutex
	customers      map[string]domain.Customer
	products       map[string]domain.Product
	orders         map[string]domain.SaleOrder
	invoices       map[string]domain.Invoice
	deliveries     map[string]domain.DeliveryOrder
	vendors        map[string]domain.Vendor
	purchaseOrders map[string]domain.PurchaseOrder
	vendorBills    map[string]domain.VendorBill
	orderSeq       int
	invSeq         int
	cnSeq          int
	delivSeq       int
	poSeq          int
	billSeq        int
	receiptSeq     int
}

func NewMockERPAdapter() *MockERPAdapter {
	adapter := &MockERPAdapter{
		customers:      make(map[string]domain.Customer),
		products:       make(map[string]domain.Product),
		orders:         make(map[string]domain.SaleOrder),
		invoices:       make(map[string]domain.Invoice),
		deliveries:     make(map[string]domain.DeliveryOrder),
		vendors:        make(map[string]domain.Vendor),
		purchaseOrders: make(map[string]domain.PurchaseOrder),
		vendorBills:    make(map[string]domain.VendorBill),
		orderSeq:       100,
		invSeq:         200,
		cnSeq:          300,
		delivSeq:       400,
		poSeq:          500,
		billSeq:        600,
		receiptSeq:     700,
	}
	adapter.seedData()
	return adapter
}

func (m *MockERPAdapter) seedData() {
	// Seed Customers
	c1 := domain.Customer{ID: "10", Name: "Azure Interior", Email: "info@azure.example.com", Phone: "+254 700 123456", CreditLimit: 15000, Balance: 2500, City: "Nairobi"}
	c2 := domain.Customer{ID: "12", Name: "Deco Addict", Email: "contact@decoaddict.com", Phone: "+254 711 987654", CreditLimit: 25000, Balance: 0, City: "Mombasa"}
	c3 := domain.Customer{ID: "15", Name: "Gemini Furniture", Email: "sales@gemini.co.ke", Phone: "+254 722 555444", CreditLimit: 50000, Balance: 12000, City: "Eldoret"}

	m.customers[c1.ID] = c1
	m.customers[c2.ID] = c2
	m.customers[c3.ID] = c3

	// Seed Products
	p1 := domain.Product{ID: "101", Name: "Customizable Desk", SKU: "DESK-001", ListPrice: 450.00, StandardPrice: 250.00, QtyAvailable: 45}
	p2 := domain.Product{ID: "102", Name: "Ergonomic Office Chair", SKU: "CHAIR-002", ListPrice: 180.00, StandardPrice: 90.00, QtyAvailable: 120}
	p3 := domain.Product{ID: "103", Name: "Acoustic Screen", SKU: "SCR-003", ListPrice: 320.00, StandardPrice: 180.00, QtyAvailable: 15}

	m.products[p1.ID] = p1
	m.products[p2.ID] = p2
	m.products[p3.ID] = p3

	// Seed Existing Sale Order / Quotation
	so1 := domain.SaleOrder{
		ID:            "501",
		OrderNo:       "SO001",
		CustomerID:    "10",
		CustomerName:  "Azure Interior",
		DateOrder:     time.Now().Add(-48 * time.Hour),
		AmountUntaxed: 900.00,
		AmountTax:     144.00,
		AmountTotal:   1044.00,
		State:         "draft",
		Lines: []domain.SaleOrderLine{
			{ID: "1", ProductID: "101", ProductName: "Customizable Desk", Quantity: 2, UnitPrice: 450.00, PriceSubtotal: 900.00},
		},
		Note:          "Initial draft quotation for Azure Interior",
		DeliveryState: "pending",
		InvoiceState:  "no",
	}
	m.orders[so1.ID] = so1

	// Seed Vendors
	v1 := domain.Vendor{ID: "20", Name: "ABC Ltd", Email: "procurement@abc.example.com", Phone: "+254 700 654321", City: "Nairobi"}
	v2 := domain.Vendor{ID: "21", Name: "Thika Suppliers", Email: "sales@thikasuppliers.co.ke", Phone: "+254 711 112233", City: "Thika"}
	m.vendors[v1.ID] = v1
	m.vendors[v2.ID] = v2

	// Seed Invoices
	inv1 := domain.Invoice{
		ID:           "201",
		InvoiceNo:    "INV/2026/0001",
		CustomerID:   "10",
		CustomerName: "Azure Interior",
		OrderID:      "501",
		MoveType:     "out_invoice",
		AmountTotal:  2500.00,
		AmountDue:    2500.00,
		Status:       "posted",
		DueDate:      time.Now().Add(-10 * 24 * time.Hour), // Overdue
		ETIMSStatus:  "pending",
	}
	m.invoices[inv1.ID] = inv1
}

func (m *MockERPAdapter) GetCustomers(ctx context.Context, filter domain.CustomerFilter) ([]domain.Customer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []domain.Customer
	searchLower := strings.ToLower(filter.Search)

	for _, c := range m.customers {
		if searchLower != "" && !strings.Contains(strings.ToLower(c.Name), searchLower) && !strings.Contains(strings.ToLower(c.Email), searchLower) {
			continue
		}
		result = append(result, c)
	}
	return result, nil
}

func (m *MockERPAdapter) GetCustomerByID(ctx context.Context, id string) (*domain.Customer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	c, ok := m.customers[id]
	if !ok {
		for _, cust := range m.customers {
			if strings.EqualFold(cust.Name, id) || strings.Contains(strings.ToLower(cust.Name), strings.ToLower(id)) {
				return &cust, nil
			}
		}
		return nil, fmt.Errorf("customer '%s' not found", id)
	}
	return &c, nil
}

func (m *MockERPAdapter) GetProducts(ctx context.Context, filter domain.ProductFilter) ([]domain.Product, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []domain.Product
	searchLower := strings.ToLower(filter.Search)

	for _, p := range m.products {
		if searchLower != "" && !strings.Contains(strings.ToLower(p.Name), searchLower) && !strings.Contains(strings.ToLower(p.SKU), searchLower) {
			continue
		}
		result = append(result, p)
	}
	return result, nil
}

func (m *MockERPAdapter) GetProductByID(ctx context.Context, id string) (*domain.Product, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, ok := m.products[id]
	if !ok {
		for _, prod := range m.products {
			if strings.EqualFold(prod.Name, id) || strings.EqualFold(prod.SKU, id) || strings.Contains(strings.ToLower(prod.Name), strings.ToLower(id)) {
				return &prod, nil
			}
		}
		return nil, fmt.Errorf("product '%s' not found", id)
	}
	return &p, nil
}

func (m *MockERPAdapter) GetSaleOrders(ctx context.Context, filter domain.SaleOrderFilter) ([]domain.SaleOrder, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []domain.SaleOrder
	searchLower := strings.ToLower(filter.Search)

	for _, o := range m.orders {
		if filter.CustomerID != "" && o.CustomerID != filter.CustomerID {
			continue
		}
		if filter.State != "" && !strings.EqualFold(o.State, filter.State) {
			continue
		}
		if searchLower != "" && !strings.Contains(strings.ToLower(o.OrderNo), searchLower) && !strings.Contains(strings.ToLower(o.CustomerName), searchLower) {
			continue
		}
		result = append(result, o)
	}
	return result, nil
}

func (m *MockERPAdapter) getSaleOrderByIDUnlocked(id string) (*domain.SaleOrder, error) {
	o, ok := m.orders[id]
	if !ok {
		for _, ord := range m.orders {
			if strings.EqualFold(ord.OrderNo, id) || strings.EqualFold(ord.ID, id) {
				return &ord, nil
			}
		}
		return nil, fmt.Errorf("sales order / quotation '%s' not found", id)
	}
	return &o, nil
}

func (m *MockERPAdapter) GetSaleOrderByID(ctx context.Context, id string) (*domain.SaleOrder, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.getSaleOrderByIDUnlocked(id)
}

func (m *MockERPAdapter) CreateQuotation(ctx context.Context, req domain.CreateSaleOrderRequest) (*domain.ActionResult, error) {
	req.IsQuotation = true
	return m.CreateSaleOrder(ctx, req)
}

func (m *MockERPAdapter) SendQuotation(ctx context.Context, quotationID string) (*domain.ActionResult, error) {
	order, err := m.GetSaleOrderByID(ctx, quotationID)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	order.State = "sent"
	m.orders[order.ID] = *order
	m.mu.Unlock()

	return &domain.ActionResult{
		Success:      true,
		Message:      fmt.Sprintf("Quotation %s sent successfully to customer %s via email!", order.OrderNo, order.CustomerName),
		RecordID:     order.ID,
		RecordNumber: order.OrderNo,
		Data:         order,
	}, nil
}

func (m *MockERPAdapter) ConvertQuotationToOrder(ctx context.Context, quotationID string, dryRun bool) (*domain.ActionResult, error) {
	return m.ConfirmSaleOrder(ctx, quotationID, dryRun)
}

func (m *MockERPAdapter) CreateSaleOrder(ctx context.Context, req domain.CreateSaleOrderRequest) (*domain.ActionResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	customer, ok := m.customers[req.CustomerID]
	if !ok {
		for _, c := range m.customers {
			if strings.EqualFold(c.Name, req.CustomerID) || strings.Contains(strings.ToLower(c.Name), strings.ToLower(req.CustomerID)) {
				customer = c
				ok = true
				break
			}
		}
		if !ok {
			return nil, fmt.Errorf("customer '%s' not found", req.CustomerID)
		}
	}

	var lines []domain.SaleOrderLine
	var untaxedTotal float64

	for idx, l := range req.Lines {
		product, ok := m.products[l.ProductID]
		if !ok {
			for _, p := range m.products {
				if strings.EqualFold(p.Name, l.ProductID) || strings.EqualFold(p.SKU, l.ProductID) || strings.Contains(strings.ToLower(p.Name), strings.ToLower(l.ProductID)) {
					product = p
					ok = true
					break
				}
			}
		}
		if !ok {
			return nil, fmt.Errorf("product '%s' not found", l.ProductID)
		}

		unitPrice := l.UnitPrice
		if unitPrice == 0 {
			unitPrice = product.ListPrice
		}
		subtotal := unitPrice * l.Quantity
		untaxedTotal += subtotal

		lines = append(lines, domain.SaleOrderLine{
			ID:            fmt.Sprintf("%d", idx+1),
			ProductID:     product.ID,
			ProductName:   product.Name,
			Quantity:      l.Quantity,
			UnitPrice:     unitPrice,
			PriceSubtotal: subtotal,
		})
	}

	tax := untaxedTotal * 0.16
	total := untaxedTotal + tax

	m.orderSeq++
	orderID := fmt.Sprintf("%d", m.orderSeq)
	orderPrefix := "SO"
	if req.IsQuotation {
		orderPrefix = "QUO"
	}
	orderNo := fmt.Sprintf("%s%03d", orderPrefix, m.orderSeq)

	state := "draft"

	order := domain.SaleOrder{
		ID:            orderID,
		OrderNo:       orderNo,
		CustomerID:    customer.ID,
		CustomerName:  customer.Name,
		DateOrder:     time.Now(),
		AmountUntaxed: untaxedTotal,
		AmountTax:     tax,
		AmountTotal:   total,
		State:         state,
		Lines:         lines,
		Note:          req.Note,
		DeliveryState: "pending",
		InvoiceState:  "no",
	}

	opName := "CREATE_SALE_ORDER"
	if req.IsQuotation {
		opName = "CREATE_QUOTATION"
	}

	if req.DryRun {
		prompt := fmt.Sprintf("Do you want to confirm and validate the creation of %s %s for %s (Total: $%.2f)?", opName, orderNo, customer.Name, total)
		return &domain.ActionResult{
			Success: true,
			Message: fmt.Sprintf("DryRun simulation successful for customer '%s'. Total: $%.2f", customer.Name, total),
			DryRun: &domain.DryRunResult{
				IsDryRun:              true,
				Operation:             opName,
				ImpactSummary:         fmt.Sprintf("Simulated Creation of %s %s for %s. Subtotal: $%.2f, Tax: $%.2f, Total: $%.2f", opName, orderNo, customer.Name, untaxedTotal, tax, total),
				RiskLevel:             domain.RiskMedium,
				RequiresConfirmation:  true,
				ConfirmationPrompt:    prompt,
				ConfirmationToken:     fmt.Sprintf("CONFIRM_%s_%s_%d", opName, customer.ID, time.Now().Unix()),
				SimulatedOrderDetails: &order,
			},
		}, nil
	}

	m.orders[orderID] = order

	return &domain.ActionResult{
		Success:      true,
		Message:      fmt.Sprintf("%s %s created successfully for %s!", opName, orderNo, customer.Name),
		RecordID:     orderID,
		RecordNumber: orderNo,
		Data:         order,
	}, nil
}

func (m *MockERPAdapter) ConfirmSaleOrder(ctx context.Context, orderID string, dryRun bool) (*domain.ActionResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var targetKey string
	var order domain.SaleOrder
	found := false

	for k, o := range m.orders {
		if strings.EqualFold(o.ID, orderID) || strings.EqualFold(o.OrderNo, orderID) {
			targetKey = k
			order = o
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("sale order / quotation '%s' not found", orderID)
	}

	if dryRun {
		prompt := fmt.Sprintf("Confirming Sales Order %s will convert state to 'sale' and reserve inventory stock for $%.2f. Do you want to proceed and validate?", order.OrderNo, order.AmountTotal)
		return &domain.ActionResult{
			Success: true,
			Message: fmt.Sprintf("DryRun simulation: Order %s (%s) is ready to be confirmed.", order.OrderNo, order.CustomerName),
			DryRun: &domain.DryRunResult{
				IsDryRun:              true,
				Operation:             "CONFIRM_SALE_ORDER",
				ImpactSummary:         fmt.Sprintf("Confirming Sales Order %s will update state from '%s' to 'sale' (Confirmed) and reserve stock.", order.OrderNo, order.State),
				RiskLevel:             domain.RiskMedium,
				RequiresConfirmation:  true,
				ConfirmationPrompt:    prompt,
				ConfirmationToken:     fmt.Sprintf("CONFIRM_SO_%s_%d", order.ID, time.Now().Unix()),
				SimulatedOrderDetails: &order,
			},
		}, nil
	}

	order.State = "sale"
	order.DeliveryState = "ready"
	m.orders[targetKey] = order

	return &domain.ActionResult{
		Success:      true,
		Message:      fmt.Sprintf("Sale Order %s confirmed successfully!", order.OrderNo),
		RecordID:     order.ID,
		RecordNumber: order.OrderNo,
		Data:         order,
	}, nil
}

func (m *MockERPAdapter) DeliverSaleOrder(ctx context.Context, orderID string, dryRun bool) (*domain.ActionResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	order, err := m.getSaleOrderByIDUnlocked(orderID)
	if err != nil {
		return nil, err
	}

	if dryRun {
		prompt := fmt.Sprintf("Are you sure you want to validate and process inventory delivery for Sales Order %s (%s)?", order.OrderNo, order.CustomerName)
		return &domain.ActionResult{
			Success: true,
			Message: fmt.Sprintf("DryRun simulation: Delivery execution preview for Order %s.", order.OrderNo),
			DryRun: &domain.DryRunResult{
				IsDryRun:              true,
				Operation:             "DELIVER_SALE_ORDER",
				ImpactSummary:         fmt.Sprintf("Executing delivery for Order %s will deduct reserved items from inventory stock and mark order as 'done'.", order.OrderNo),
				RiskLevel:             domain.RiskHigh,
				RequiresConfirmation:  true,
				ConfirmationPrompt:    prompt,
				ConfirmationToken:     fmt.Sprintf("CONFIRM_DELIVERY_%s_%d", order.ID, time.Now().Unix()),
				SimulatedOrderDetails: order,
			},
		}, nil
	}

	order.State = "done"
	order.DeliveryState = "fully_delivered"
	order.InvoiceState = "to_invoice"
	m.orders[order.ID] = *order

	m.delivSeq++
	delivNo := fmt.Sprintf("WH/OUT/%05d", m.delivSeq)

	return &domain.ActionResult{
		Success:      true,
		Message:      fmt.Sprintf("Delivery %s for Sales Order %s validated and completed! Inventory stock updated.", delivNo, order.OrderNo),
		RecordID:     fmt.Sprintf("%d", m.delivSeq),
		RecordNumber: delivNo,
		Data:         order,
	}, nil
}

func (m *MockERPAdapter) CancelSaleOrder(ctx context.Context, orderID string, dryRun bool) (*domain.ActionResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	order, err := m.getSaleOrderByIDUnlocked(orderID)
	if err != nil {
		return nil, err
	}

	if dryRun {
		prompt := fmt.Sprintf("CAUTION: Cancelling Sales Order %s will void the order and release reserved inventory. Do you confirm cancellation?", order.OrderNo)
		return &domain.ActionResult{
			Success: true,
			Message: fmt.Sprintf("DryRun simulation: Order %s cancellation preview.", order.OrderNo),
			DryRun: &domain.DryRunResult{
				IsDryRun:              true,
				Operation:             "CANCEL_SALE_ORDER",
				ImpactSummary:         fmt.Sprintf("Canceling Sales Order %s will void the order and release stock.", order.OrderNo),
				RiskLevel:             domain.RiskHigh,
				RequiresConfirmation:  true,
				ConfirmationPrompt:    prompt,
				ConfirmationToken:     fmt.Sprintf("CONFIRM_CANCEL_%s_%d", order.ID, time.Now().Unix()),
				SimulatedOrderDetails: order,
			},
		}, nil
	}

	order.State = "cancel"
	m.orders[order.ID] = *order

	return &domain.ActionResult{
		Success:      true,
		Message:      fmt.Sprintf("Sale Order %s has been cancelled.", order.OrderNo),
		RecordID:     order.ID,
		RecordNumber: order.OrderNo,
		Data:         order,
	}, nil
}

func (m *MockERPAdapter) CreateInvoice(ctx context.Context, orderID string, dryRun bool) (*domain.ActionResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	order, err := m.getSaleOrderByIDUnlocked(orderID)
	if err != nil {
		return nil, err
	}

	m.invSeq++
	invID := fmt.Sprintf("%d", m.invSeq)
	invNo := fmt.Sprintf("INV/2026/%04d", m.invSeq)

	inv := domain.Invoice{
		ID:           invID,
		InvoiceNo:    invNo,
		CustomerID:   order.CustomerID,
		CustomerName: order.CustomerName,
		OrderID:      order.ID,
		MoveType:     "out_invoice",
		AmountTotal:  order.AmountTotal,
		AmountDue:    order.AmountTotal,
		Status:       "draft",
		DueDate:      time.Now().Add(30 * 24 * time.Hour),
		ETIMSStatus:  "pending",
	}

	if dryRun {
		prompt := fmt.Sprintf("Draft Invoice %s for $%.2f (%s) is ready. Do you want to validate and generate this invoice?", invNo, inv.AmountTotal, order.CustomerName)
		return &domain.ActionResult{
			Success: true,
			Message: fmt.Sprintf("DryRun simulation: Draft Invoice %s preview.", invNo),
			DryRun: &domain.DryRunResult{
				IsDryRun:             true,
				Operation:            "CREATE_INVOICE",
				ImpactSummary:        fmt.Sprintf("Drafting customer invoice %s for Sales Order %s (Total: $%.2f).", invNo, order.OrderNo, order.AmountTotal),
				RiskLevel:            domain.RiskMedium,
				RequiresConfirmation: true,
				ConfirmationPrompt:   prompt,
				ConfirmationToken:    fmt.Sprintf("CONFIRM_INV_%s_%d", invID, time.Now().Unix()),
				SimulatedInvoice:     &inv,
			},
		}, nil
	}

	m.invoices[invID] = inv
	return &domain.ActionResult{
		Success:      true,
		Message:      fmt.Sprintf("Draft Invoice %s created successfully for Sales Order %s!", invNo, order.OrderNo),
		RecordID:     invID,
		RecordNumber: invNo,
		Data:         inv,
	}, nil
}

func (m *MockERPAdapter) PostInvoice(ctx context.Context, invoiceID string, dryRun bool) (*domain.ActionResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	inv, ok := m.invoices[invoiceID]
	if !ok {
		for _, i := range m.invoices {
			if strings.EqualFold(i.InvoiceNo, invoiceID) {
				inv = i
				ok = true
				break
			}
		}
		if !ok {
			return nil, fmt.Errorf("invoice '%s' not found", invoiceID)
		}
	}

	if dryRun {
		prompt := fmt.Sprintf("Posting Invoice %s will finalize accounting entry for $%.2f. Confirm posting?", inv.InvoiceNo, inv.AmountTotal)
		return &domain.ActionResult{
			Success: true,
			Message: fmt.Sprintf("DryRun simulation: Posting Invoice %s preview.", inv.InvoiceNo),
			DryRun: &domain.DryRunResult{
				IsDryRun:             true,
				Operation:            "POST_INVOICE",
				ImpactSummary:        fmt.Sprintf("Posting Invoice %s will finalize accounting entry.", inv.InvoiceNo),
				RiskLevel:            domain.RiskHigh,
				RequiresConfirmation: true,
				ConfirmationPrompt:   prompt,
				SimulatedInvoice:     &inv,
			},
		}, nil
	}

	inv.Status = "posted"
	m.invoices[inv.ID] = inv

	return &domain.ActionResult{
		Success:      true,
		Message:      fmt.Sprintf("Invoice %s posted successfully!", inv.InvoiceNo),
		RecordID:     inv.ID,
		RecordNumber: inv.InvoiceNo,
		Data:         inv,
	}, nil
}

func (m *MockERPAdapter) CreateCreditNote(ctx context.Context, req domain.CreditNoteRequest) (*domain.ActionResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	inv, ok := m.invoices[req.InvoiceID]
	if !ok {
		for _, i := range m.invoices {
			if strings.EqualFold(i.InvoiceNo, req.InvoiceID) {
				inv = i
				ok = true
				break
			}
		}
		if !ok {
			return nil, fmt.Errorf("origin invoice '%s' not found", req.InvoiceID)
		}
	}

	refundAmount := req.Amount
	if refundAmount <= 0 || refundAmount > inv.AmountTotal {
		refundAmount = inv.AmountTotal
	}

	m.cnSeq++
	cnID := fmt.Sprintf("%d", m.cnSeq)
	cnNo := fmt.Sprintf("RINV/2026/%04d", m.cnSeq)

	cn := domain.Invoice{
		ID:           cnID,
		InvoiceNo:    cnNo,
		CustomerID:   inv.CustomerID,
		CustomerName: inv.CustomerName,
		OrderID:      inv.OrderID,
		MoveType:     "out_refund",
		AmountTotal:  refundAmount,
		AmountDue:    0,
		Status:       "posted",
		DueDate:      time.Now(),
		ETIMSStatus:  "fiscalized",
	}

	if req.DryRun {
		prompt := fmt.Sprintf("HIGH IMPACT: Creating Credit Note %s for $%.2f against Invoice %s (Reason: %s). Do you confirm this credit refund?", cnNo, refundAmount, inv.InvoiceNo, req.Reason)
		return &domain.ActionResult{
			Success: true,
			Message: fmt.Sprintf("DryRun simulation: Credit Note %s preview.", cnNo),
			DryRun: &domain.DryRunResult{
				IsDryRun:             true,
				Operation:            "CREATE_CREDIT_NOTE",
				ImpactSummary:        fmt.Sprintf("Credit Note %s will reverse $%.2f from Invoice %s.", cnNo, refundAmount, inv.InvoiceNo),
				RiskLevel:            domain.RiskHigh,
				RequiresConfirmation: true,
				ConfirmationPrompt:   prompt,
				ConfirmationToken:    fmt.Sprintf("CONFIRM_CN_%s_%d", cnID, time.Now().Unix()),
				SimulatedInvoice:     &cn,
			},
		}, nil
	}

	m.invoices[cnID] = cn

	return &domain.ActionResult{
		Success:      true,
		Message:      fmt.Sprintf("Credit Note %s issued successfully for Invoice %s ($%.2f refund)!", cnNo, inv.InvoiceNo, refundAmount),
		RecordID:     cnID,
		RecordNumber: cnNo,
		Data:         cn,
	}, nil
}

func (m *MockERPAdapter) SubmitETIMS(ctx context.Context, invoiceID string, dryRun bool) (*domain.ETIMSSubmissionResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	inv, ok := m.invoices[invoiceID]
	if !ok {
		for _, i := range m.invoices {
			if strings.EqualFold(i.InvoiceNo, invoiceID) {
				inv = i
				ok = true
				break
			}
		}
		if !ok {
			return nil, fmt.Errorf("invoice '%s' not found for eTIMS submission", invoiceID)
		}
	}

	cuSerial := fmt.Sprintf("KRA%09d", time.Now().Unix()%1000000000)
	irn := fmt.Sprintf("IRN-2026-%s-%04d", inv.CustomerID, m.invSeq)
	qrURL := fmt.Sprintf("https://etims.kra.go.ke/verify?irn=%s", irn)

	if dryRun {
		return &domain.ETIMSSubmissionResult{
			InvoiceID:        inv.ID,
			InvoiceNo:        inv.InvoiceNo,
			FiscalStatus:     "DRY_RUN_PREVIEW",
			CUSerialNo:       cuSerial,
			IRN:              irn,
			QRCodeURL:        qrURL,
			FiscalizedAmount: inv.AmountTotal,
			TaxAmount:        inv.AmountTotal * 0.16,
			Timestamp:        time.Now(),
			Message:          fmt.Sprintf("DryRun Preview: Invoice %s is compliant and ready for KRA eTIMS fiscalization.", inv.InvoiceNo),
		}, nil
	}

	inv.ETIMSStatus = "fiscalized"
	inv.CUSerialNo = cuSerial
	inv.IRN = irn
	inv.QRCodeURL = qrURL
	m.invoices[inv.ID] = inv

	return &domain.ETIMSSubmissionResult{
		InvoiceID:        inv.ID,
		InvoiceNo:        inv.InvoiceNo,
		FiscalStatus:     "SUCCESS",
		CUSerialNo:       cuSerial,
		IRN:              irn,
		QRCodeURL:        qrURL,
		FiscalizedAmount: inv.AmountTotal,
		TaxAmount:        inv.AmountTotal * 0.16,
		Timestamp:        time.Now(),
		Message:          fmt.Sprintf("Invoice %s successfully fiscalized with KRA eTIMS! CU Serial: %s, IRN: %s", inv.InvoiceNo, cuSerial, irn),
	}, nil
}

func (m *MockERPAdapter) PrintDocument(ctx context.Context, req domain.PrintDocumentRequest) (*domain.PrintDocumentResult, error) {
	docType := strings.ToLower(req.DocumentType)
	docNo := fmt.Sprintf("DOC-%s-%s", docType, req.DocumentID)
	downloadURL := fmt.Sprintf("http://localhost:3000/api/v1/documents/%s/%s/pdf", docType, req.DocumentID)

	return &domain.PrintDocumentResult{
		DocumentType: req.DocumentType,
		DocumentID:   req.DocumentID,
		DocumentNo:   docNo,
		Format:       "pdf",
		DownloadURL:  downloadURL,
		PDFBase64:    "JVBERi0xLjQKJ...[MOCK_PDF_BINARY_PAYLOAD]...",
	}, nil
}

func (m *MockERPAdapter) GetInvoices(ctx context.Context, filter domain.InvoiceFilter) ([]domain.Invoice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []domain.Invoice
	for _, inv := range m.invoices {
		if filter.CustomerID != "" && inv.CustomerID != filter.CustomerID {
			continue
		}
		if filter.MoveType != "" && !strings.EqualFold(inv.MoveType, filter.MoveType) {
			continue
		}
		if filter.Status != "" && !strings.EqualFold(inv.Status, filter.Status) {
			continue
		}
		result = append(result, inv)
	}
	return result, nil
}

func (m *MockERPAdapter) GetOverdueInvoices(ctx context.Context, minAmount float64) ([]domain.Invoice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var overdue []domain.Invoice
	now := time.Now()
	for _, inv := range m.invoices {
		if inv.AmountDue >= minAmount && inv.DueDate.Before(now) {
			overdue = append(overdue, inv)
		}
	}
	return overdue, nil
}

func (m *MockERPAdapter) GetStockLevel(ctx context.Context, sku string, locationID string) (float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, p := range m.products {
		if strings.EqualFold(p.SKU, sku) || strings.EqualFold(p.ID, sku) {
			return p.QtyAvailable, nil
		}
	}
	return 0, fmt.Errorf("product SKU %s not found", sku)
}

func (m *MockERPAdapter) TransferStock(ctx context.Context, req domain.StockTransferRequest) (*domain.ActionResult, error) {
	if req.DryRun {
		return &domain.ActionResult{
			Success: true,
			Message: fmt.Sprintf("DryRun simulation: Transfer of %.2f units of SKU %s", req.Quantity, req.ProductSKU),
			DryRun: &domain.DryRunResult{
				IsDryRun:             true,
				Operation:            "TRANSFER_STOCK",
				ImpactSummary:        fmt.Sprintf("Transferring %.2f units of SKU %s from location %s to %s", req.Quantity, req.ProductSKU, req.FromLocID, req.ToLocID),
				RiskLevel:            domain.RiskMedium,
				RequiresConfirmation: true,
			},
		}, nil
	}

	return &domain.ActionResult{
		Success: true,
		Message: fmt.Sprintf("Stock transfer of %.2f units of SKU %s completed.", req.Quantity, req.ProductSKU),
	}, nil
}

// ---- Purchase Operations (portable; Odoo mapping lives in adapters/odoo) ----

func (m *MockERPAdapter) GetVendors(ctx context.Context, filter domain.VendorFilter) ([]domain.Vendor, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []domain.Vendor
	searchLower := strings.ToLower(filter.Search)

	for _, v := range m.vendors {
		if searchLower != "" && !strings.Contains(strings.ToLower(v.Name), searchLower) && !strings.Contains(strings.ToLower(v.Email), searchLower) {
			continue
		}
		if filter.Email != "" && !strings.EqualFold(v.Email, filter.Email) {
			continue
		}
		result = append(result, v)
	}
	return result, nil
}

func (m *MockERPAdapter) GetVendorByID(ctx context.Context, id string) (*domain.Vendor, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	v, ok := m.vendors[id]
	if !ok {
		for _, vend := range m.vendors {
			if strings.EqualFold(vend.Name, id) || strings.Contains(strings.ToLower(vend.Name), strings.ToLower(id)) {
				return &vend, nil
			}
		}
		return nil, fmt.Errorf("vendor '%s' not found", id)
	}
	return &v, nil
}

func (m *MockERPAdapter) getPurchaseOrderByIDUnlocked(id string) (*domain.PurchaseOrder, error) {
	o, ok := m.purchaseOrders[id]
	if !ok {
		for _, ord := range m.purchaseOrders {
			if strings.EqualFold(ord.OrderNo, id) || strings.EqualFold(ord.ID, id) {
				return &ord, nil
			}
		}
		return nil, fmt.Errorf("purchase order / RFQ '%s' not found", id)
	}
	return &o, nil
}

func (m *MockERPAdapter) GetPurchaseOrders(ctx context.Context, filter domain.PurchaseOrderFilter) ([]domain.PurchaseOrder, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []domain.PurchaseOrder
	searchLower := strings.ToLower(filter.Search)

	for _, o := range m.purchaseOrders {
		if filter.VendorID != "" && o.VendorID != filter.VendorID {
			continue
		}
		if filter.State != "" && !strings.EqualFold(o.State, filter.State) {
			continue
		}
		if searchLower != "" && !strings.Contains(strings.ToLower(o.OrderNo), searchLower) && !strings.Contains(strings.ToLower(o.VendorName), searchLower) {
			continue
		}
		result = append(result, o)
	}
	return result, nil
}

func (m *MockERPAdapter) GetPurchaseOrderByID(ctx context.Context, id string) (*domain.PurchaseOrder, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.getPurchaseOrderByIDUnlocked(id)
}

func (m *MockERPAdapter) CreateRFQ(ctx context.Context, req domain.CreatePurchaseOrderRequest) (*domain.ActionResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	vendor, ok := m.vendors[req.VendorID]
	if !ok {
		for _, v := range m.vendors {
			if strings.EqualFold(v.Name, req.VendorID) || strings.Contains(strings.ToLower(v.Name), strings.ToLower(req.VendorID)) {
				vendor = v
				ok = true
				break
			}
		}
		if !ok {
			return nil, fmt.Errorf("vendor '%s' not found", req.VendorID)
		}
	}

	var lines []domain.PurchaseOrderLine
	var untaxedTotal float64

	for idx, l := range req.Lines {
		product, ok := m.products[l.ProductID]
		if !ok {
			for _, p := range m.products {
				if strings.EqualFold(p.Name, l.ProductID) || strings.EqualFold(p.SKU, l.ProductID) || strings.Contains(strings.ToLower(p.Name), strings.ToLower(l.ProductID)) {
					product = p
					ok = true
					break
				}
			}
		}
		if !ok {
			return nil, fmt.Errorf("product '%s' not found", l.ProductID)
		}

		unitPrice := l.UnitPrice
		if unitPrice == 0 {
			unitPrice = product.StandardPrice
			if unitPrice == 0 {
				unitPrice = product.ListPrice
			}
		}
		subtotal := unitPrice * l.Quantity
		untaxedTotal += subtotal

		lines = append(lines, domain.PurchaseOrderLine{
			ID:            fmt.Sprintf("%d", idx+1),
			ProductID:     product.ID,
			ProductName:   product.Name,
			Quantity:      l.Quantity,
			UnitPrice:     unitPrice,
			PriceSubtotal: subtotal,
		})
	}

	tax := untaxedTotal * 0.16
	total := untaxedTotal + tax

	m.poSeq++
	orderID := fmt.Sprintf("%d", m.poSeq)
	orderNo := fmt.Sprintf("PO%05d", m.poSeq)

	order := domain.PurchaseOrder{
		ID:            orderID,
		OrderNo:       orderNo,
		VendorID:      vendor.ID,
		VendorName:    vendor.Name,
		DateOrder:     time.Now(),
		AmountUntaxed: untaxedTotal,
		AmountTax:     tax,
		AmountTotal:   total,
		State:         "draft",
		Lines:         lines,
		Note:          req.Note,
		ReceiptState:  "pending",
		BillingState:  "no",
	}

	if req.DryRun {
		return &domain.ActionResult{
			Success: true,
			Message: fmt.Sprintf("DryRun simulation: RFQ %s for %s (Total: $%.2f) is ready to draft.", orderNo, vendor.Name, total),
			DryRun: &domain.DryRunResult{
				IsDryRun:              true,
				Operation:             "CREATE_RFQ",
				ImpactSummary:         fmt.Sprintf("Simulated creation of RFQ %s for %s. Subtotal: $%.2f, Tax: $%.2f, Total: $%.2f", orderNo, vendor.Name, untaxedTotal, tax, total),
				RiskLevel:             domain.RiskLow,
				RequiresConfirmation:  true,
				ConfirmationPrompt:    fmt.Sprintf("Do you want to draft RFQ %s for %s (Total: $%.2f)?", orderNo, vendor.Name, total),
				ConfirmationToken:     fmt.Sprintf("CONFIRM_RFQ_%s_%d", vendor.ID, time.Now().Unix()),
				SimulatedOrderDetails: nil,
			},
		}, nil
	}

	m.purchaseOrders[orderID] = order

	return &domain.ActionResult{
		Success:      true,
		Message:      fmt.Sprintf("RFQ %s drafted successfully for %s!", orderNo, vendor.Name),
		RecordID:     orderID,
		RecordNumber: orderNo,
		Data:         order,
	}, nil
}

func (m *MockERPAdapter) ConfirmPurchaseOrder(ctx context.Context, orderID string, dryRun bool) (*domain.ActionResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	order, err := m.getPurchaseOrderByIDUnlocked(orderID)
	if err != nil {
		return nil, err
	}

	if order.State != "draft" && order.State != "sent" && order.State != "rfq" {
		return nil, fmt.Errorf("purchase order %s is in state '%s' and cannot be confirmed (must be draft/RFQ)", order.OrderNo, order.State)
	}

	if dryRun {
		return &domain.ActionResult{
			Success: true,
			Message: fmt.Sprintf("DryRun simulation: PO %s (%s) is ready to confirm.", order.OrderNo, order.VendorName),
			DryRun: &domain.DryRunResult{
				IsDryRun:              true,
				Operation:             "CONFIRM_PURCHASE_ORDER",
				ImpactSummary:         fmt.Sprintf("Confirming PO %s will update state from '%s' to 'purchase' (Confirmed), committing $%.2f spend with %s.", order.OrderNo, order.State, order.AmountTotal, order.VendorName),
				RiskLevel:             domain.RiskMedium,
				RequiresConfirmation:  true,
				ConfirmationPrompt:    fmt.Sprintf("Confirming PO %s commits $%.2f with %s. Do you want to proceed?", order.OrderNo, order.AmountTotal, order.VendorName),
				ConfirmationToken:     fmt.Sprintf("CONFIRM_PO_%s_%d", order.ID, time.Now().Unix()),
			},
		}, nil
	}

	order.State = "purchase"
	order.ReceiptState = "pending"
	order.BillingState = "to_bill"
	m.purchaseOrders[order.ID] = *order

	return &domain.ActionResult{
		Success:      true,
		Message:      fmt.Sprintf("Purchase Order %s confirmed successfully!", order.OrderNo),
		RecordID:     order.ID,
		RecordNumber: order.OrderNo,
		Data:         order,
	}, nil
}

func (m *MockERPAdapter) CancelPurchaseOrder(ctx context.Context, orderID string, dryRun bool) (*domain.ActionResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	order, err := m.getPurchaseOrderByIDUnlocked(orderID)
	if err != nil {
		return nil, err
	}

	if order.State != "draft" && order.State != "sent" && order.State != "rfq" {
		return nil, fmt.Errorf("purchase order %s is in state '%s' and cannot be cancelled in v1 (draft/RFQ only)", order.OrderNo, order.State)
	}

	if dryRun {
		return &domain.ActionResult{
			Success: true,
			Message: fmt.Sprintf("DryRun simulation: RFQ %s cancellation preview.", order.OrderNo),
			DryRun: &domain.DryRunResult{
				IsDryRun:              true,
				Operation:             "CANCEL_PURCHASE_ORDER",
				ImpactSummary:         fmt.Sprintf("Cancelling RFQ %s will void the draft request to %s.", order.OrderNo, order.VendorName),
				RiskLevel:             domain.RiskMedium,
				RequiresConfirmation:  true,
				ConfirmationPrompt:    fmt.Sprintf("CAUTION: Cancel RFQ %s to %s? This voids the draft.", order.OrderNo, order.VendorName),
				ConfirmationToken:     fmt.Sprintf("CONFIRM_CANCEL_PO_%s_%d", order.ID, time.Now().Unix()),
			},
		}, nil
	}

	order.State = "cancel"
	m.purchaseOrders[order.ID] = *order

	return &domain.ActionResult{
		Success:      true,
		Message:      fmt.Sprintf("RFQ %s has been cancelled.", order.OrderNo),
		RecordID:     order.ID,
		RecordNumber: order.OrderNo,
		Data:         order,
	}, nil
}

func (m *MockERPAdapter) ReceivePurchaseOrder(ctx context.Context, orderID string, dryRun bool) (*domain.ActionResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	order, err := m.getPurchaseOrderByIDUnlocked(orderID)
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
				IsDryRun:              true,
				Operation:             "RECEIVE_PURCHASE_ORDER",
				ImpactSummary:         fmt.Sprintf("Receiving PO %s will validate the full inbound picking and increase inventory stock (v1: full receipt only, no partials/backorders).", order.OrderNo),
				RiskLevel:             domain.RiskHigh,
				RequiresConfirmation:  true,
				ConfirmationPrompt:    fmt.Sprintf("Receive ALL lines of PO %s from %s into stock? (Full receipt only.)", order.OrderNo, order.VendorName),
				ConfirmationToken:     fmt.Sprintf("CONFIRM_RECEIPT_%s_%d", order.ID, time.Now().Unix()),
			},
		}, nil
	}

	order.State = "done"
	order.ReceiptState = "fully_received"
	m.purchaseOrders[order.ID] = *order

	m.receiptSeq++
	receiptNo := fmt.Sprintf("WH/IN/%05d", m.receiptSeq)

	return &domain.ActionResult{
		Success:      true,
		Message:      fmt.Sprintf("Receipt %s for PO %s validated and completed! Inventory stock updated.", receiptNo, order.OrderNo),
		RecordID:     fmt.Sprintf("%d", m.receiptSeq),
		RecordNumber: receiptNo,
		Data:         order,
	}, nil
}

func (m *MockERPAdapter) GetVendorBills(ctx context.Context, filter domain.VendorBillFilter) ([]domain.VendorBill, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []domain.VendorBill
	for _, b := range m.vendorBills {
		if filter.VendorID != "" && b.VendorID != filter.VendorID {
			continue
		}
		if filter.MoveType != "" && !strings.EqualFold(b.MoveType, filter.MoveType) {
			continue
		}
		if filter.Status != "" && !strings.EqualFold(b.Status, filter.Status) {
			continue
		}
		result = append(result, b)
	}
	return result, nil
}

func (m *MockERPAdapter) CreateVendorBill(ctx context.Context, orderID string, dryRun bool) (*domain.ActionResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	order, err := m.getPurchaseOrderByIDUnlocked(orderID)
	if err != nil {
		return nil, err
	}

	if order.State != "purchase" && order.State != "done" {
		return nil, fmt.Errorf("purchase order %s is in state '%s'; bill only after confirm/receive", order.OrderNo, order.State)
	}

	m.billSeq++
	billID := fmt.Sprintf("%d", m.billSeq)
	billNo := fmt.Sprintf("BILL/2026/%04d", m.billSeq)

	bill := domain.VendorBill{
		ID:          billID,
		BillNo:      billNo,
		VendorID:    order.VendorID,
		VendorName:  order.VendorName,
		OrderID:     order.ID,
		MoveType:    "in_invoice",
		AmountTotal: order.AmountTotal,
		AmountDue:   order.AmountTotal,
		Status:      "draft",
		DueDate:     time.Now().Add(30 * 24 * time.Hour),
	}

	if dryRun {
		return &domain.ActionResult{
			Success: true,
			Message: fmt.Sprintf("DryRun simulation: Vendor bill %s preview (auto-copied from PO %s).", billNo, order.OrderNo),
			DryRun: &domain.DryRunResult{
				IsDryRun:              true,
				Operation:             "CREATE_VENDOR_BILL",
				ImpactSummary:         fmt.Sprintf("Drafting vendor bill %s from PO %s (Total: $%.2f, auto-copied lines, no edits in v1).", billNo, order.OrderNo, order.AmountTotal),
				RiskLevel:             domain.RiskMedium,
				RequiresConfirmation:  true,
				ConfirmationPrompt:    fmt.Sprintf("Draft vendor bill %s for $%.2f from PO %s?", billNo, bill.AmountTotal, order.OrderNo),
				ConfirmationToken:     fmt.Sprintf("CONFIRM_BILL_%s_%d", billID, time.Now().Unix()),
				SimulatedInvoice:      nil,
			},
		}, nil
	}

	m.vendorBills[billID] = bill
	order.BillingState = "billed"
	m.purchaseOrders[order.ID] = *order

	return &domain.ActionResult{
		Success:      true,
		Message:      fmt.Sprintf("Vendor bill %s created successfully from PO %s!", billNo, order.OrderNo),
		RecordID:     billID,
		RecordNumber: billNo,
		Data:         bill,
	}, nil
}

func (m *MockERPAdapter) PostVendorBill(ctx context.Context, billID string, dryRun bool) (*domain.ActionResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	bill, ok := m.vendorBills[billID]
	if !ok {
		for _, b := range m.vendorBills {
			if strings.EqualFold(b.BillNo, billID) {
				bill = b
				ok = true
				break
			}
		}
		if !ok {
			return nil, fmt.Errorf("vendor bill '%s' not found", billID)
		}
	}

	if dryRun {
		return &domain.ActionResult{
			Success: true,
			Message: fmt.Sprintf("DryRun simulation: Posting vendor bill %s preview.", bill.BillNo),
			DryRun: &domain.DryRunResult{
				IsDryRun:              true,
				Operation:             "POST_VENDOR_BILL",
				ImpactSummary:         fmt.Sprintf("Posting vendor bill %s will finalize accounting entry for $%.2f (post-only in v1, no payment execution).", bill.BillNo, bill.AmountTotal),
				RiskLevel:             domain.RiskHigh,
				RequiresConfirmation:  true,
				ConfirmationPrompt:    fmt.Sprintf("Post vendor bill %s for $%.2f? (Posts journal only, no payment.)", bill.BillNo, bill.AmountTotal),
				ConfirmationToken:     fmt.Sprintf("CONFIRM_POST_BILL_%s_%d", bill.ID, time.Now().Unix()),
			},
		}, nil
	}

	bill.Status = "posted"
	m.vendorBills[bill.ID] = bill

	return &domain.ActionResult{
		Success:      true,
		Message:      fmt.Sprintf("Vendor bill %s posted successfully!", bill.BillNo),
		RecordID:     bill.ID,
		RecordNumber: bill.BillNo,
		Data:         bill,
	}, nil
}
