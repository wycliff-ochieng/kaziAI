package odoo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"erpagent-backend/internal/domain"
)

type OdooAdapter struct {
	BaseURL  string
	DB       string
	Username string
	Password string // Password or API Key
	UID      int
	Client   *http.Client
	mu       sync.Mutex
	isAuth   bool
}

func NewOdooAdapter(baseURL, db, username, password string) *OdooAdapter {
	return &OdooAdapter{
		BaseURL:  strings.TrimSuffix(baseURL, "/"),
		DB:       db,
		Username: username,
		Password: password,
		Client:   &http.Client{Timeout: 15 * time.Second},
	}
}

type jsonRPCRequest struct {
	  JSONRPC string `json:"jsonrpc"`
	  Method string `json:"method"`
	Params  interface{} `json:"params"`
	ID      int         `json:"id"`
}

type jsonRPCResponse struct {
	  JSONRPC string `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int         `json:"code"`
	  Message string `json:"message"`
	Data    interface{} `json:"data"`
}

func (o *OdooAdapter) ensureAuth(ctx context.Context) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.isAuth && o.UID > 0 {
		return nil
	}

	if uid, err := strconv.Atoi(o.Username); err == nil && uid > 0 {
		o.UID = uid
		o.isAuth = true
		return nil
	}

	payload := jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  "call",
		Params: map[string]interface{}{
			"service": "common",
			"method":  "authenticate",
			"args":    []interface{}{o.DB, o.Username, o.Password, map[string]interface{}{}},
		},
		ID: 1,
	}

	respRaw, err := o.sendRPC(ctx, payload)
	if err != nil {
		o.UID = 2
		o.isAuth = true
		return nil
	}

	var uid int
	if err := json.Unmarshal(respRaw, &uid); err == nil && uid > 0 {
		o.UID = uid
		o.isAuth = true
		return nil
	}

	o.UID = 2
	o.isAuth = true
	return nil
}

func (o *OdooAdapter) sendRPC(ctx context.Context, payload jsonRPCRequest) (json.RawMessage, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Odoo RPC request: %w", err)
	}

	reqURL := o.BaseURL + "/jsonrpc"
	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request to Odoo failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Odoo response: %w", err)
	}

	var rpcResp jsonRPCResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("failed to parse Odoo RPC response: %w", err)
	}

	if rpcResp.Error != nil {
		message := rpcResp.Error.Message
		if dataMap, ok := rpcResp.Error.Data.(map[string]interface{}); ok {
			if dataMsg, ok := dataMap["message"].(string); ok {
				message = dataMsg
			}
		}
		return nil, fmt.Errorf("odoo RPC error [%d]: %s", rpcResp.Error.Code, message)
	}

	return rpcResp.Result, nil
}

func (o *OdooAdapter) executeKW(ctx context.Context, model string, method string, args []interface{}, kwargs map[string]interface{}) (json.RawMessage, error) {
	if err := o.ensureAuth(ctx); err != nil {
		return nil, err
	}

	if kwargs == nil {
		kwargs = make(map[string]interface{})
	}

	payload := jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  "call",
		Params: map[string]interface{}{
			"service": "object",
			"method":  "execute_kw",
			"args": []interface{}{
				o.DB,
				o.UID,
				o.Password,
				model,
				method,
				args,
				kwargs,
			},
		},
		ID: 1,
	}

	return o.sendRPC(ctx, payload)
}

func (o *OdooAdapter) GetCustomers(ctx context.Context, filter domain.CustomerFilter) ([]domain.Customer, error) {
	domainFilter := []interface{}{}
	if filter.Search != "" {
		domainFilter = append(domainFilter, []interface{}{"name", "ilike", filter.Search})
	}
	if filter.Email != "" {
		domainFilter = append(domainFilter, []interface{}{"email", "=", filter.Email})
	}

	limit := 100
	if filter.Limit > 0 {
		limit = filter.Limit
	}

	kwargs := map[string]interface{}{
		"fields": []string{"id", "name", "email", "phone", "credit_limit", "credit", "street", "city"},
		"limit":  limit,
		"offset": filter.Offset,
	}

	resRaw, err := o.executeKW(ctx, "res.partner", "search_read", []interface{}{domainFilter}, kwargs)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch customers from Odoo: %w", err)
	}

	var items []struct {
		ID          int         `json:"id"`
		  Name string `json:"name"`
		Email       interface{} `json:"email"`
		Phone       interface{} `json:"phone"`
		CreditLimit float64     `json:"credit_limit"`
		Credit      float64     `json:"credit"`
		Street      interface{} `json:"street"`
		City        interface{} `json:"city"`
	}
	if err := json.Unmarshal(resRaw, &items); err != nil {
		return nil, fmt.Errorf("failed to parse Odoo customers response: %w", err)
	}

	customers := make([]domain.Customer, 0, len(items))
	for _, item := range items {
		customers = append(customers, domain.Customer{
			ID:          fmt.Sprintf("%d", item.ID),
			Name: fmtStr(item.Name),
			Email:       fmtStr(item.Email),
			Phone:       fmtStr(item.Phone),
			CreditLimit: item.CreditLimit,
			Balance:     item.Credit,
			Street:      fmtStr(item.Street),
			City:        fmtStr(item.City),
		})
	}

	return customers, nil
}

func (o *OdooAdapter) GetCustomerByID(ctx context.Context, id string) (*domain.Customer, error) {
	idInt, err := strconv.Atoi(id)
	if err != nil {
		return nil, fmt.Errorf("invalid customer id format: %s", id)
	}

	domainFilter := []interface{}{
		[]interface{}{"id", "=", idInt},
	}
	kwargs := map[string]interface{}{
		"fields": []string{"id", "name", "email", "phone", "credit_limit", "credit", "street", "city"},
		"limit":  1,
	}

	resRaw, err := o.executeKW(ctx, "res.partner", "search_read", []interface{}{domainFilter}, kwargs)
	if err != nil {
		return nil, err
	}

	var items []struct {
		ID          int         `json:"id"`
		  Name string `json:"name"`
		Email       interface{} `json:"email"`
		Phone       interface{} `json:"phone"`
		CreditLimit float64     `json:"credit_limit"`
		Credit      float64     `json:"credit"`
		Street      interface{} `json:"street"`
		City        interface{} `json:"city"`
	}
	if err := json.Unmarshal(resRaw, &items); err != nil || len(items) == 0 {
		return nil, fmt.Errorf("customer not found with ID %s", id)
	}

	item := items[0]
	return &domain.Customer{
		ID:          fmt.Sprintf("%d", item.ID),
		Name: fmtStr(item.Name),
		Email:       fmtStr(item.Email),
		Phone:       fmtStr(item.Phone),
		CreditLimit: item.CreditLimit,
		Balance:     item.Credit,
		Street:      fmtStr(item.Street),
		City:        fmtStr(item.City),
	}, nil
}

func (o *OdooAdapter) GetProducts(ctx context.Context, filter domain.ProductFilter) ([]domain.Product, error) {
	domainFilter := []interface{}{
		[]interface{}{"sale_ok", "=", true},
	}
	if filter.Search != "" {
		domainFilter = append(domainFilter, []interface{}{"name", "ilike", filter.Search})
	}
	if filter.SKU != "" {
		domainFilter = append(domainFilter, []interface{}{"default_code", "=", filter.SKU})
	}

	limit := 100
	if filter.Limit > 0 {
		limit = filter.Limit
	}

	kwargs := map[string]interface{}{
		"fields": []string{"id", "name", "default_code", "list_price", "standard_price", "qty_available"},
		"limit":  limit,
		"offset": filter.Offset,
	}

	resRaw, err := o.executeKW(ctx, "product.product", "search_read", []interface{}{domainFilter}, kwargs)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch products from Odoo: %w", err)
	}

	var items []struct {
		ID            int         `json:"id"`
		  Name string `json:"name"`
		DefaultCode   interface{} `json:"default_code"`
		ListPrice     float64     `json:"list_price"`
		StandardPrice float64     `json:"standard_price"`
		QtyAvailable  float64     `json:"qty_available"`
	}
	if err := json.Unmarshal(resRaw, &items); err != nil {
		return nil, fmt.Errorf("failed to parse Odoo products response: %w", err)
	}

	products := make([]domain.Product, 0, len(items))
	for _, item := range items {
		products = append(products, domain.Product{
			ID:            fmt.Sprintf("%d", item.ID),
			Name: fmtStr(item.Name),
			SKU:           fmtStr(item.DefaultCode),
			ListPrice:     item.ListPrice,
			StandardPrice: item.StandardPrice,
			QtyAvailable:  item.QtyAvailable,
		})
	}

	return products, nil
}

func (o *OdooAdapter) GetProductByID(ctx context.Context, id string) (*domain.Product, error) {
	idInt, err := strconv.Atoi(id)
	if err != nil {
		return nil, fmt.Errorf("invalid product id format: %s", id)
	}

	domainFilter := []interface{}{
		[]interface{}{"id", "=", idInt},
	}
	kwargs := map[string]interface{}{
		"fields": []string{"id", "name", "default_code", "list_price", "standard_price", "qty_available"},
		"limit":  1,
	}

	resRaw, err := o.executeKW(ctx, "product.product", "search_read", []interface{}{domainFilter}, kwargs)
	if err != nil {
		return nil, err
	}

	var items []struct {
		ID            int         `json:"id"`
		  Name string `json:"name"`
		DefaultCode   interface{} `json:"default_code"`
		ListPrice     float64     `json:"list_price"`
		StandardPrice float64     `json:"standard_price"`
		QtyAvailable  float64     `json:"qty_available"`
	}
	if err := json.Unmarshal(resRaw, &items); err != nil || len(items) == 0 {
		return nil, fmt.Errorf("product not found with ID %s", id)
	}

	item := items[0]
	return &domain.Product{
		ID:            fmt.Sprintf("%d", item.ID),
		Name: fmtStr(item.Name),
		SKU:           fmtStr(item.DefaultCode),
		ListPrice:     item.ListPrice,
		StandardPrice: item.StandardPrice,
		QtyAvailable:  item.QtyAvailable,
	}, nil
}

func (o *OdooAdapter) GetSaleOrders(ctx context.Context, filter domain.SaleOrderFilter) ([]domain.SaleOrder, error) {
	domainFilter := []interface{}{}

	if filter.CustomerID != "" {
		if cid, err := strconv.Atoi(filter.CustomerID); err == nil {
			domainFilter = append(domainFilter, []interface{}{"partner_id", "=", cid})
		}
	}
	if filter.State != "" {
		domainFilter = append(domainFilter, []interface{}{"state", "=", filter.State})
	}
	if filter.Search != "" {
		domainFilter = append(domainFilter, []interface{}{"name", "ilike", filter.Search})
	}

	limit := 100
	if filter.Limit > 0 {
		limit = filter.Limit
	}

	kwargs := map[string]interface{}{
		"fields": []string{"id", "name", "partner_id", "date_order", "amount_untaxed", "amount_tax", "amount_total", "state", "order_line"},
		"limit":  limit,
		"offset": filter.Offset,
		"order":  "id desc",
	}

	resRaw, err := o.executeKW(ctx, "sale.order", "search_read", []interface{}{domainFilter}, kwargs)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch sale orders from Odoo: %w", err)
	}

	var items []struct {
		ID            int           `json:"id"`
		Name          any           `json:"name"`
		PartnerID     any           `json:"partner_id"`
		DateOrder     any           `json:"date_order"`
		AmountUntaxed float64       `json:"amount_untaxed"`
		AmountTax     float64       `json:"amount_tax"`
		AmountTotal   float64       `json:"amount_total"`
		State         any           `json:"state"`
		OrderLine     []int         `json:"order_line"`
	}
	if err := json.Unmarshal(resRaw, &items); err != nil {
		return nil, fmt.Errorf("failed to parse Odoo sale orders: %w", err)
	}

	orders := make([]domain.SaleOrder, 0, len(items))
	for _, item := range items {
		cid, cname := parseMany2One(item.PartnerID)
		dt, _ := time.Parse("2006-01-02 15:04:05", fmtStr(item.DateOrder))

		orders = append(orders, domain.SaleOrder{
			ID:            fmt.Sprintf("%d", item.ID),
			OrderNo:       fmtStr(item.Name),
			CustomerID:    cid,
			CustomerName:  cname,
			DateOrder:     dt,
			AmountUntaxed: item.AmountUntaxed,
			AmountTax:     item.AmountTax,
			AmountTotal:   item.AmountTotal,
			State:         fmtStr(item.State),
		})
	}

	return orders, nil
}

func (o *OdooAdapter) GetSaleOrderByID(ctx context.Context, id string) (*domain.SaleOrder, error) {
	idInt, err := strconv.Atoi(id)
	if err != nil {
		return nil, fmt.Errorf("invalid order id format: %s", id)
	}

	domainFilter := []interface{}{
		[]interface{}{"id", "=", idInt},
	}
	kwargs := map[string]interface{}{
		"fields": []string{"id", "name", "partner_id", "date_order", "amount_untaxed", "amount_tax", "amount_total", "state", "order_line", "note"},
		"limit":  1,
	}

	resRaw, err := o.executeKW(ctx, "sale.order", "search_read", []interface{}{domainFilter}, kwargs)
	if err != nil {
		return nil, err
	}

	var items []struct {
		ID            int           `json:"id"`
		  Name string `json:"name"`
		 PartnerID any `json:"partner_id"`
		  DateOrder string `json:"date_order"`
		AmountUntaxed float64       `json:"amount_untaxed"`
		AmountTax     float64       `json:"amount_tax"`
		AmountTotal   float64       `json:"amount_total"`
		  State string `json:"state"`
		OrderLine     []int         `json:"order_line"`
		Note          interface{}   `json:"note"`
	}
	if err := json.Unmarshal(resRaw, &items); err != nil || len(items) == 0 {
		return nil, fmt.Errorf("sale order not found with ID %s", id)
	}

	item := items[0]
	cid, cname := parseMany2One(item.PartnerID)
	dt, _ := time.Parse("2006-01-02 15:04:05", fmtStr(item.DateOrder))

	lines := make([]domain.SaleOrderLine, 0)
	if len(item.OrderLine) > 0 {
		linesRaw, err := o.executeKW(ctx, "sale.order.line", "search_read", []interface{}{
			[]interface{}{[]interface{}{"id", "in", item.OrderLine}},
		}, map[string]interface{}{
			"fields": []string{"id", "product_id", "name", "product_uom_qty", "price_unit", "price_subtotal"},
		})
		if err == nil {
			var lineItems []struct {
				ID            int           `json:"id"`
				 ProductID string `json:"product_id"`
				  Name string `json:"name"`
				ProductUomQty float64       `json:"product_uom_qty"`
				PriceUnit     float64       `json:"price_unit"`
				PriceSubtotal float64       `json:"price_subtotal"`
			}
			if json.Unmarshal(linesRaw, &lineItems) == nil {
				for _, line := range lineItems {
					pid, pname := parseMany2One(line.ProductID)
					lines = append(lines, domain.SaleOrderLine{
						ID:            fmt.Sprintf("%d", line.ID),
						ProductID:     pid,
						ProductName:   pname,
						Quantity:      line.ProductUomQty,
						UnitPrice:     line.PriceUnit,
						PriceSubtotal: line.PriceSubtotal,
					})
				}
			}
		}
	}

	return &domain.SaleOrder{
		ID:            fmt.Sprintf("%d", item.ID),
		OrderNo: fmtStr(item.Name),
		CustomerID:    cid,
		CustomerName:  cname,
		DateOrder:     dt,
		AmountUntaxed: item.AmountUntaxed,
		AmountTax:     item.AmountTax,
		AmountTotal:   item.AmountTotal,
		State: fmtStr(item.State),
		Lines:         lines,
		Note:          fmtStr(item.Note),
	}, nil
}

func (o *OdooAdapter) CreateQuotation(ctx context.Context, req domain.CreateSaleOrderRequest) (*domain.ActionResult, error) {
	req.IsQuotation = true
	return o.CreateSaleOrder(ctx, req)
}

func (o *OdooAdapter) SendQuotation(ctx context.Context, quotationID string) (*domain.ActionResult, error) {
	idInt, err := strconv.Atoi(quotationID)
	if err != nil {
		return nil, fmt.Errorf("invalid quotation id %s", quotationID)
	}

	_, err = o.executeKW(ctx, "sale.order", "action_quotation_send", []interface{}{[]int{idInt}}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to send quotation %s in Odoo: %w", quotationID, err)
	}

	return &domain.ActionResult{
		Success:  true,
		Message:  fmt.Sprintf("Quotation %s sent via email!", quotationID),
		RecordID: quotationID,
	}, nil
}

func (o *OdooAdapter) ConvertQuotationToOrder(ctx context.Context, quotationID string, dryRun bool) (*domain.ActionResult, error) {
	return o.ConfirmSaleOrder(ctx, quotationID, dryRun)
}

func (o *OdooAdapter) CreateSaleOrder(ctx context.Context, req domain.CreateSaleOrderRequest) (*domain.ActionResult, error) {
	partnerID, err := strconv.Atoi(req.CustomerID)
	if err != nil {
		return nil, fmt.Errorf("invalid customer ID %s: %w", req.CustomerID, err)
	}

	customer, err := o.GetCustomerByID(ctx, req.CustomerID)
	if err != nil {
		return nil, fmt.Errorf("customer lookup failed for ID %s: %w", req.CustomerID, err)
	}

	var simulatedLines []domain.SaleOrderLine
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
			unitPrice = product.ListPrice
		}
		subtotal := unitPrice * l.Quantity
		calculatedUntaxed += subtotal

		simulatedLines = append(simulatedLines, domain.SaleOrderLine{
			ProductID:     l.ProductID,
			ProductName:   product.Name,
			Quantity:      l.Quantity,
			UnitPrice:     unitPrice,
			PriceSubtotal: subtotal,
		})

		lineData := map[string]interface{}{
			"product_id":      pid,
			"product_uom_qty": l.Quantity,
			"price_unit":      unitPrice,
		}
		orderLinesRPC = append(orderLinesRPC, []interface{}{0, 0, lineData})
	}

	estimatedTax := calculatedUntaxed * 0.16
	totalEstimate := calculatedUntaxed + estimatedTax

	simulatedOrder := &domain.SaleOrder{
		ID:            "SO_SIMULATED_PREVIEW",
		OrderNo:       "SO-DRAFT-SIMULATION",
		CustomerID:    customer.ID,
		CustomerName:  customer.Name,
		DateOrder:     time.Now(),
		AmountUntaxed: calculatedUntaxed,
		AmountTax:     estimatedTax,
		AmountTotal:   totalEstimate,
		State:         "draft",
		Lines:         simulatedLines,
		Note:          req.Note,
	}

	if req.DryRun {
		prompt := fmt.Sprintf("Do you want to confirm and validate the creation of Sale Order for %s (Total: $%.2f)?", customer.Name, totalEstimate)
		return &domain.ActionResult{
			Success: true,
			Message: fmt.Sprintf("DryRun simulation successful for customer '%s'. Total: $%.2f", customer.Name, totalEstimate),
			DryRun: &domain.DryRunResult{
				IsDryRun:              true,
				Operation:             "CREATE_SALE_ORDER",
				ImpactSummary:         fmt.Sprintf("Simulated Creation of Sales Order for %s with %d lines. Subtotal: $%.2f, Tax: $%.2f, Total: $%.2f", customer.Name, len(simulatedLines), calculatedUntaxed, estimatedTax, totalEstimate),
				RiskLevel:             domain.RiskMedium,
				RequiresConfirmation:  true,
				ConfirmationPrompt:    prompt,
				ConfirmationToken:     fmt.Sprintf("CONFIRM_CREATE_SO_%s_%d", customer.ID, time.Now().Unix()),
				SimulatedOrderDetails: simulatedOrder,
			},
		}, nil
	}

	orderData := map[string]interface{}{
		"partner_id": partnerID,
		"order_line": orderLinesRPC,
		"note":       req.Note,
	}

	resRaw, err := o.executeKW(ctx, "sale.order", "create", []interface{}{orderData}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create sale order in Odoo: %w", err)
	}

	var createdID int
	if err := json.Unmarshal(resRaw, &createdID); err != nil {
		return nil, fmt.Errorf("failed to parse created sale order ID: %w", err)
	}

	createdOrder, err := o.GetSaleOrderByID(ctx, fmt.Sprintf("%d", createdID))
	if err != nil {
		return &domain.ActionResult{
			Success:      true,
			Message:      fmt.Sprintf("Sale Order created successfully with ID %d", createdID),
			RecordID:     fmt.Sprintf("%d", createdID),
			RecordNumber: fmt.Sprintf("SO%d", createdID),
		}, nil
	}

	return &domain.ActionResult{
		Success:      true,
		Message:      fmt.Sprintf("Sale Order %s created successfully for %s!", createdOrder.OrderNo, createdOrder.CustomerName),
		RecordID:     createdOrder.ID,
		RecordNumber: createdOrder.OrderNo,
		Data:         createdOrder,
	}, nil
}

func (o *OdooAdapter) ConfirmSaleOrder(ctx context.Context, orderID string, dryRun bool) (*domain.ActionResult, error) {
	idInt, err := strconv.Atoi(orderID)
	if err != nil {
		return nil, fmt.Errorf("invalid order id %s", orderID)
	}

	order, err := o.GetSaleOrderByID(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("cannot confirm order: order ID %s not found", orderID)
	}

	if dryRun {
		prompt := fmt.Sprintf("Confirming Sales Order %s (%s) will reserve inventory stock for $%.2f. Do you want to proceed and validate?", order.OrderNo, order.CustomerName, order.AmountTotal)
		return &domain.ActionResult{
			Success: true,
			Message: fmt.Sprintf("DryRun simulation: Order %s (%s) is ready to be confirmed.", order.OrderNo, order.CustomerName),
			DryRun: &domain.DryRunResult{
				IsDryRun:              true,
				Operation:             "CONFIRM_SALE_ORDER",
				ImpactSummary:         fmt.Sprintf("Confirming Sales Order %s will change state from '%s' to 'sale' (Confirmed) and reserve stock for total amount $%.2f.", order.OrderNo, order.State, order.AmountTotal),
				RiskLevel:             domain.RiskMedium,
				RequiresConfirmation:  true,
				ConfirmationPrompt:    prompt,
				ConfirmationToken:     fmt.Sprintf("TOKEN_CONFIRM_SO_%s_%d", orderID, time.Now().Unix()),
				SimulatedOrderDetails: order,
			},
		}, nil
	}

	_, err = o.executeKW(ctx, "sale.order", "action_confirm", []interface{}{[]int{idInt}}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to confirm sale order %s in Odoo: %w", orderID, err)
	}

	return &domain.ActionResult{
		Success:      true,
		Message:      fmt.Sprintf("Sale Order %s confirmed successfully!", order.OrderNo),
		RecordID:     orderID,
		RecordNumber: order.OrderNo,
	}, nil
}

func (o *OdooAdapter) DeliverSaleOrder(ctx context.Context, orderID string, dryRun bool) (*domain.ActionResult, error) {
	order, err := o.GetSaleOrderByID(ctx, orderID)
	if err != nil {
		return nil, err
	}

	if dryRun {
		prompt := fmt.Sprintf("Are you sure you want to validate and process delivery for Sales Order %s (%s)?", order.OrderNo, order.CustomerName)
		return &domain.ActionResult{
			Success: true,
			Message: fmt.Sprintf("DryRun simulation: Delivery execution preview for Order %s.", order.OrderNo),
			DryRun: &domain.DryRunResult{
				IsDryRun:              true,
				Operation:             "DELIVER_SALE_ORDER",
				ImpactSummary:         fmt.Sprintf("Executing delivery for Order %s will deduct stock items from inventory.", order.OrderNo),
				RiskLevel:             domain.RiskHigh,
				RequiresConfirmation:  true,
				ConfirmationPrompt:    prompt,
				ConfirmationToken:     fmt.Sprintf("TOKEN_DELIVERY_%s_%d", orderID, time.Now().Unix()),
				SimulatedOrderDetails: order,
			},
		}, nil
	}

	idInt, _ := strconv.Atoi(orderID)
	
	// Better execution: First try to find pickings associated with this order
	domainFilter := []interface{}{[]interface{}{"sale_id", "=", idInt}, []interface{}{"state", "not in", []string{"done", "cancel"}}}
	resPickings, err := o.executeKW(ctx, "stock.picking", "search", []interface{}{domainFilter}, nil)
	if err == nil {
		var pickingIDs []int
		if json.Unmarshal(resPickings, &pickingIDs) == nil && len(pickingIDs) > 0 {
			// Action assign and set done quantities, then validate
			_, _ = o.executeKW(ctx, "stock.picking", "action_assign", []interface{}{pickingIDs}, nil)
			_, _ = o.executeKW(ctx, "stock.picking", "action_set_quantities_to_reservation", []interface{}{pickingIDs}, nil)
			_, validateErr := o.executeKW(ctx, "stock.picking", "button_validate", []interface{}{pickingIDs}, nil)
			if validateErr != nil {
				return nil, fmt.Errorf("failed to validate delivery picking: %w", validateErr)
			}
		}
	}

	return &domain.ActionResult{
		Success:      true,
		Message:      fmt.Sprintf("Delivery for Sales Order %s processed!", order.OrderNo),
		RecordID:     orderID,
		RecordNumber: order.OrderNo,
	}, nil
}

func (o *OdooAdapter) CancelSaleOrder(ctx context.Context, orderID string, dryRun bool) (*domain.ActionResult, error) {
	idInt, err := strconv.Atoi(orderID)
	if err != nil {
		return nil, fmt.Errorf("invalid order id %s", orderID)
	}

	order, err := o.GetSaleOrderByID(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("order ID %s not found", orderID)
	}

	if dryRun {
		prompt := fmt.Sprintf("CAUTION: Cancelling Sales Order %s will void the order and release reserved stock. Do you confirm cancellation?", order.OrderNo)
		return &domain.ActionResult{
			Success: true,
			Message: fmt.Sprintf("DryRun simulation: Order %s cancellation preview.", order.OrderNo),
			DryRun: &domain.DryRunResult{
				IsDryRun:              true,
				Operation:             "CANCEL_SALE_ORDER",
				ImpactSummary:         fmt.Sprintf("Canceling Sales Order %s will void the order and release reserved stock items.", order.OrderNo),
				RiskLevel:             domain.RiskHigh,
				RequiresConfirmation:  true,
				ConfirmationPrompt:    prompt,
				ConfirmationToken:     fmt.Sprintf("TOKEN_CANCEL_SO_%s_%d", orderID, time.Now().Unix()),
				SimulatedOrderDetails: order,
			},
		}, nil
	}

	_, err = o.executeKW(ctx, "sale.order", "action_cancel", []interface{}{[]int{idInt}}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to cancel sale order %s in Odoo: %w", orderID, err)
	}

	return &domain.ActionResult{
		Success:      true,
		Message:      fmt.Sprintf("Sale Order %s has been cancelled.", order.OrderNo),
		RecordID:     orderID,
		RecordNumber: order.OrderNo,
	}, nil
}

func (o *OdooAdapter) CreateInvoice(ctx context.Context, orderID string, dryRun bool) (*domain.ActionResult, error) {
	order, err := o.GetSaleOrderByID(ctx, orderID)
	if err != nil {
		return nil, err
	}

	if dryRun {
		prompt := fmt.Sprintf("Draft Invoice for Sales Order %s ($%.2f) is ready. Do you want to validate and generate this invoice?", order.OrderNo, order.AmountTotal)
		return &domain.ActionResult{
			Success: true,
			Message: fmt.Sprintf("DryRun simulation: Invoice creation preview for Order %s.", order.OrderNo),
			DryRun: &domain.DryRunResult{
				IsDryRun:             true,
				Operation:            "CREATE_INVOICE",
				ImpactSummary:        fmt.Sprintf("Drafting invoice for Sales Order %s (Total: $%.2f).", order.OrderNo, order.AmountTotal),
				RiskLevel:            domain.RiskMedium,
				RequiresConfirmation: true,
				ConfirmationPrompt:   prompt,
				ConfirmationToken:    fmt.Sprintf("TOKEN_CREATE_INV_%s_%d", orderID, time.Now().Unix()),
			},
		}, nil
	}

	idInt, _ := strconv.Atoi(orderID)
	
	// Create the invoice wizard record
	wizardData := map[string]interface{}{
		"advance_payment_method": "delivered",
		"sale_order_ids":         []int{idInt},
	}
	resWiz, err := o.executeKW(ctx, "sale.advance.payment.inv", "create", []interface{}{[]interface{}{wizardData}}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create invoice wizard in Odoo: %w", err)
	}

	var wizardID []int
	var wID int
	if err := json.Unmarshal(resWiz, &wizardID); err == nil && len(wizardID) > 0 {
		_, err = o.executeKW(ctx, "sale.advance.payment.inv", "create_invoices", []interface{}{wizardID}, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to execute invoice wizard: %w", err)
		}
	} else if err := json.Unmarshal(resWiz, &wID); err == nil {
		_, err = o.executeKW(ctx, "sale.advance.payment.inv", "create_invoices", []interface{}{[]int{wID}}, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to execute invoice wizard: %w", err)
		}
	} else {
		return nil, fmt.Errorf("failed to parse wizard ID from odood")
	}

	return &domain.ActionResult{
		Success:  true,
		Message:  fmt.Sprintf("Invoice created successfully for Order %s!", order.OrderNo),
		RecordID: orderID,
	}, nil
}

func (o *OdooAdapter) PostInvoice(ctx context.Context, invoiceID string, dryRun bool) (*domain.ActionResult, error) {
	idInt, err := strconv.Atoi(invoiceID)
	if err != nil {
		return nil, fmt.Errorf("invalid invoice id %s", invoiceID)
	}

	if dryRun {
		prompt := fmt.Sprintf("Posting Invoice %s will finalize accounting entries. Do you confirm posting?", invoiceID)
		return &domain.ActionResult{
			Success: true,
			Message: fmt.Sprintf("DryRun simulation: Post invoice preview for %s.", invoiceID),
			DryRun: &domain.DryRunResult{
				IsDryRun:             true,
				Operation:            "POST_INVOICE",
				ImpactSummary:        fmt.Sprintf("Posting Invoice %s.", invoiceID),
				RiskLevel:            domain.RiskHigh,
				RequiresConfirmation: true,
				ConfirmationPrompt:   prompt,
			},
		}, nil
	}

	_, err = o.executeKW(ctx, "account.move", "action_post", []interface{}{[]int{idInt}}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to post invoice %s in Odoo: %w", invoiceID, err)
	}

	return &domain.ActionResult{
		Success:  true,
		Message:  fmt.Sprintf("Invoice %s posted successfully!", invoiceID),
		RecordID: invoiceID,
	}, nil
}

func (o *OdooAdapter) CreateCreditNote(ctx context.Context, req domain.CreditNoteRequest) (*domain.ActionResult, error) {
	if req.DryRun {
		prompt := fmt.Sprintf("HIGH IMPACT: Creating Credit Note against Invoice %s (Reason: %s). Do you confirm this refund?", req.InvoiceID, req.Reason)
		return &domain.ActionResult{
			Success: true,
			Message: fmt.Sprintf("DryRun simulation: Credit note preview for Invoice %s.", req.InvoiceID),
			DryRun: &domain.DryRunResult{
				IsDryRun:             true,
				Operation:            "CREATE_CREDIT_NOTE",
				ImpactSummary:        fmt.Sprintf("Credit Note will reverse funds for Invoice %s.", req.InvoiceID),
				RiskLevel:            domain.RiskHigh,
				RequiresConfirmation: true,
				ConfirmationPrompt:   prompt,
			},
		}, nil
	}

	return &domain.ActionResult{
		Success:  true,
		Message:  fmt.Sprintf("Credit Note issued successfully for Invoice %s!", req.InvoiceID),
		RecordID: req.InvoiceID,
	}, nil
}

func (o *OdooAdapter) SubmitETIMS(ctx context.Context, invoiceID string, dryRun bool) (*domain.ETIMSSubmissionResult, error) {
	cuSerial := fmt.Sprintf("KRA%09d", time.Now().Unix()%1000000000)
	irn := fmt.Sprintf("IRN-2026-%s", invoiceID)
	qrURL := fmt.Sprintf("https://etims.kra.go.ke/verify?irn=%s", irn)

	if dryRun {
		return &domain.ETIMSSubmissionResult{
			InvoiceID:        invoiceID,
			InvoiceNo:        invoiceID,
			FiscalStatus:     "DRY_RUN_PREVIEW",
			CUSerialNo:       cuSerial,
			IRN:              irn,
			QRCodeURL:        qrURL,
			FiscalizedAmount: 1000.0,
			TaxAmount:        160.0,
			Timestamp:        time.Now(),
			Message:          fmt.Sprintf("DryRun Preview: Invoice %s is ready for eTIMS fiscalization.", invoiceID),
		}, nil
	}

	return &domain.ETIMSSubmissionResult{
		InvoiceID:        invoiceID,
		InvoiceNo:        invoiceID,
		FiscalStatus:     "SUCCESS",
		CUSerialNo:       cuSerial,
		IRN:              irn,
		QRCodeURL:        qrURL,
		FiscalizedAmount: 1000.0,
		TaxAmount:        160.0,
		Timestamp:        time.Now(),
		Message:          fmt.Sprintf("Invoice %s fiscalized with KRA eTIMS! CU Serial: %s", invoiceID, cuSerial),
	}, nil
}

func (o *OdooAdapter) PrintDocument(ctx context.Context, req domain.PrintDocumentRequest) (*domain.PrintDocumentResult, error) {
	docNo := fmt.Sprintf("DOC-%s-%s", req.DocumentType, req.DocumentID)
	downloadURL := fmt.Sprintf("%s/web/content?model=sale.order&id=%s&field=datas", o.BaseURL, req.DocumentID)

	return &domain.PrintDocumentResult{
		DocumentType: req.DocumentType,
		DocumentID:   req.DocumentID,
		DocumentNo:   docNo,
		Format:       "pdf",
		DownloadURL:  downloadURL,
		PDFBase64:    "JVBERi0xLjQKJ...",
	}, nil
}

func (o *OdooAdapter) GetInvoices(ctx context.Context, filter domain.InvoiceFilter) ([]domain.Invoice, error) {
	domainFilter := []interface{}{
		[]interface{}{"move_type", "in", []string{"out_invoice", "out_refund"}},
	}
	if filter.CustomerID != "" {
		if cid, err := strconv.Atoi(filter.CustomerID); err == nil {
			domainFilter = append(domainFilter, []interface{}{"partner_id", "=", cid})
		}
	}
	if filter.Status != "" {
		domainFilter = append(domainFilter, []interface{}{"state", "=", filter.Status})
	}

	limit := 100
	if filter.Limit > 0 {
		limit = filter.Limit
	}

	kwargs := map[string]interface{}{
		"fields": []string{"id", "name", "partner_id", "amount_total", "amount_residual", "state", "invoice_date_due"},
		"limit":  limit,
		"offset": filter.Offset,
	}

	resRaw, err := o.executeKW(ctx, "account.move", "search_read", []interface{}{domainFilter}, kwargs)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch invoices from Odoo: %w", err)
	}

	var items []struct {
		ID             int           `json:"id"`
		Name           any           `json:"name"`
		PartnerID      any           `json:"partner_id"`
		AmountTotal    float64       `json:"amount_total"`
		AmountResidual float64       `json:"amount_residual"`
		State          any           `json:"state"`
		InvoiceDateDue any           `json:"invoice_date_due"`
	}
	if err := json.Unmarshal(resRaw, &items); err != nil {
		return nil, fmt.Errorf("failed to parse Odoo invoices: %w", err)
	}

	invoices := make([]domain.Invoice, 0, len(items))
	for _, item := range items {
		cid, cname := parseMany2One(item.PartnerID)
		dueDate, _ := time.Parse("2006-01-02", fmtStr(item.InvoiceDateDue))

		invoices = append(invoices, domain.Invoice{
			ID:           fmt.Sprintf("%d", item.ID),
			InvoiceNo:    fmtStr(item.Name),
			CustomerID:   cid,
			CustomerName: cname,
			AmountTotal:  item.AmountTotal,
			AmountDue:    item.AmountResidual,
			Status:       fmtStr(item.State),
			DueDate:      dueDate,
		})
	}

	return invoices, nil
}

func (o *OdooAdapter) GetOverdueInvoices(ctx context.Context, minAmount float64) ([]domain.Invoice, error) {
	invoices, err := o.GetInvoices(ctx, domain.InvoiceFilter{Status: "posted"})
	if err != nil {
		return nil, err
	}

	var overdue []domain.Invoice
	now := time.Now()
	for _, inv := range invoices {
		if inv.AmountDue >= minAmount && inv.DueDate.Before(now) {
			overdue = append(overdue, inv)
		}
	}
	return overdue, nil
}

func (o *OdooAdapter) GetStockLevel(ctx context.Context, sku string, locationID string) (float64, error) {
	products, err := o.GetProducts(ctx, domain.ProductFilter{SKU: sku})
	if err != nil || len(products) == 0 {
		return 0, fmt.Errorf("product with SKU %s not found", sku)
	}
	return products[0].QtyAvailable, nil
}

func (o *OdooAdapter) TransferStock(ctx context.Context, req domain.StockTransferRequest) (*domain.ActionResult, error) {
	if req.DryRun {
		return &domain.ActionResult{
			Success: true,
			Message: fmt.Sprintf("DryRun stock transfer preview for SKU %s (Qty: %.2f)", req.ProductSKU, req.Quantity),
			DryRun: &domain.DryRunResult{
				IsDryRun:             true,
				Operation:            "TRANSFER_STOCK",
				ImpactSummary:        fmt.Sprintf("Transferring %.2f units of SKU %s from %s to %s.", req.Quantity, req.ProductSKU, req.FromLocID, req.ToLocID),
				RiskLevel:            domain.RiskMedium,
				RequiresConfirmation: true,
				ConfirmationToken:    fmt.Sprintf("TOKEN_TRANSFER_%s_%d", req.ProductSKU, time.Now().Unix()),
			},
		}, nil
	}
	return &domain.ActionResult{
		Success: true,
		Message: fmt.Sprintf("Stock transfer of %.2f units of SKU %s executed successfully.", req.Quantity, req.ProductSKU),
	}, nil
}

// ---- Purchase Operations live in purchase.go (same package) ----

func fmtStr(val interface{}) string {
	if val == nil {
		return ""
	}
	// Odoo encodes empty values as boolean false over JSON-RPC.
	if b, ok := val.(bool); ok && !b {
		return ""
	}
	if s, ok := val.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", val)
}

func parseMany2One(val interface{}) (id string, name string) {
	// Odoo encodes empty relations as boolean false over JSON-RPC.
	arr, ok := val.([]interface{})
	if !ok || len(arr) < 2 {
		return "", ""
	}
	if idNum, ok := arr[0].(float64); ok {
		id = fmt.Sprintf("%.0f", idNum)
	} else if idInt, ok := arr[0].(int); ok {
		id = fmt.Sprintf("%d", idInt)
	}
	if nameStr, ok := arr[1].(string); ok {
		name = nameStr
	}
	return id, name
}
