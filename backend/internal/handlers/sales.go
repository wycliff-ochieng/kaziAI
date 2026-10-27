package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"erpagent-backend/internal/config"
	"erpagent-backend/internal/connectors"
	"erpagent-backend/internal/domain"
	"erpagent-backend/internal/factory"
	"erpagent-backend/middleware"
	"github.com/gorilla/mux"
)

var GlobalConnStore connectors.Store

func getConnector(r *http.Request, cfg *config.Config) (domain.ERPConnector, error) {
	tenantID, _ := r.Context().Value(middleware.TenantIDKey).(string)
	if tenantID == "" {
		tenantID = "default_tenant"
	}

	// An explicit erp_type is a caller-scoped override (used by tests and by
	// the AI engine when a conversation is pinned to MOCK). It must win over
	// the tenant's stored default connector.
	erpType := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("erp_type")))
	if erpType != "" {
		return factory.GetERPConnector(factory.TenantConfig{
			TenantID: tenantID,
			ERPType:  erpType,
			BaseURL:  cfg.OdooURL,
			DBName:   cfg.OdooDB,
			Username: cfg.OdooUser,
			Password: cfg.OdooPassword,
		})
	}

	if GlobalConnStore != nil {
		if stored, err := GlobalConnStore.Get(r.Context(), tenantID, "default"); err == nil && stored != nil {
			return factory.GetERPConnector(factory.TenantConfig{
				TenantID: stored.TenantID,
				ERPType:  stored.ERPType,
				BaseURL:  stored.BaseURL,
				DBName:   stored.DBName,
				Username: stored.Username,
				Password: stored.Password,
			})
		}
	}

	return factory.GetERPConnector(factory.TenantConfig{
		TenantID: tenantID,
		BaseURL:  cfg.OdooURL,
		DBName:   cfg.OdooDB,
		Username: cfg.OdooUser,
		Password: cfg.OdooPassword,
	})
}

func GetCustomers(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		connector, err := getConnector(r, cfg)
		if err != nil {
			RespondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		search := r.URL.Query().Get("search")
		email := r.URL.Query().Get("email")
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit <= 0 {
			limit = 50
		}

		customers, err := connector.GetCustomers(r.Context(), domain.CustomerFilter{
			Search: search,
			Email:  email,
			Limit:  limit,
		})
		if err != nil {
			RespondError(w, http.StatusBadRequest, err.Error())
			return
		}

		RespondJSON(w, http.StatusOK, map[string]interface{}{
			"status": "success",
			"count":  len(customers),
			"data":   customers,
		})
	}
}

func GetProducts(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		connector, err := getConnector(r, cfg)
		if err != nil {
			RespondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		search := r.URL.Query().Get("search")
		sku := r.URL.Query().Get("sku")
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit <= 0 {
			limit = 50
		}

		products, err := connector.GetProducts(r.Context(), domain.ProductFilter{
			Search: search,
			SKU:    sku,
			Limit:  limit,
		})
		if err != nil {
			RespondError(w, http.StatusBadRequest, err.Error())
			return
		}

		RespondJSON(w, http.StatusOK, map[string]interface{}{
			"status": "success",
			"count":  len(products),
			"data":   products,
		})
	}
}

func GetSaleOrders(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		connector, err := getConnector(r, cfg)
		if err != nil {
			RespondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		q := r.URL.Query()
		customerID := q.Get("customer_id")
		customerName := q.Get("customer_name")
		state := q.Get("state")
		search := q.Get("search")
		relationship := q.Get("relationship")
		limit := queryInt(q.Get("limit"), 20, 100)
		offset := queryInt(q.Get("offset"), 0, 100000)

		if relationship != "" && relationship != "orders_for_customer" {
			RespondError(w, http.StatusBadRequest, "unsupported relationship: "+relationship)
			return
		}

		if customerID == "" && customerName != "" {
			id, candidates, err := resolveCustomerID(r.Context(), connector, customerName)
			if err != nil {
				RespondError(w, http.StatusBadGateway, "customer resolution failed: "+err.Error())
				return
			}
			if id == "" {
				status := http.StatusNotFound
				code := "ENTITY_NOT_FOUND"
				if len(candidates) > 1 {
					status = http.StatusConflict
					code = "AMBIGUOUS_ENTITY"
				}
				RespondJSON(w, status, map[string]any{
					"error":            code,
					"field":            "customer",
					"message":          fmt.Sprintf("could not resolve customer %q", customerName),
					"candidates":       candidates,
					"suggested_action": "choose_one",
				})
				return
			}
			customerID = id
		}

		filter := domain.SaleOrderFilter{
			CustomerID: customerID,
			State:      state,
			Search:     search,
			Limit:      limit,
			Offset:     offset,
		}
		orders, err := connector.GetSaleOrders(r.Context(), filter)
		if err != nil {
			RespondError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := verifySaleOrders(orders, filter); err != nil {
			RespondError(w, http.StatusBadGateway, err.Error())
			return
		}

		RespondJSON(w, http.StatusOK, map[string]interface{}{
			"status": "success",
			"count":  len(orders),
			"data":   orders,
			"metadata": map[string]any{
				"limit":  limit,
				"offset": offset,
			},
		})
	}
}

func GetSaleOrderByID(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		connector, err := getConnector(r, cfg)
		if err != nil {
			RespondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		vars := mux.Vars(r)
		id := vars["id"]

		order, err := connector.GetSaleOrderByID(r.Context(), id)
		if err != nil {
			RespondError(w, http.StatusNotFound, err.Error())
			return
		}

		RespondJSON(w, http.StatusOK, map[string]interface{}{
			"status": "success",
			"data":   order,
		})
	}
}

func CreateSaleOrder(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		connector, err := getConnector(r, cfg)
		if err != nil {
			RespondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		var req domain.CreateSaleOrderRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			RespondError(w, http.StatusBadRequest, "Invalid order payload")
			return
		}

		res, err := connector.CreateSaleOrder(r.Context(), req)
		if err != nil {
			RespondError(w, http.StatusBadRequest, err.Error())
			return
		}

		RespondJSON(w, http.StatusOK, res)
	}
}

func ConfirmSaleOrder(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		connector, err := getConnector(r, cfg)
		if err != nil {
			RespondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		vars := mux.Vars(r)
		id := vars["id"]
		dryRun := r.URL.Query().Get("dry_run") == "true"

		res, err := connector.ConfirmSaleOrder(r.Context(), id, dryRun)
		if err != nil {
			RespondError(w, http.StatusBadRequest, err.Error())
			return
		}

		RespondJSON(w, http.StatusOK, res)
	}
}

func CancelSaleOrder(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		connector, err := getConnector(r, cfg)
		if err != nil {
			RespondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		vars := mux.Vars(r)
		id := vars["id"]
		dryRun := r.URL.Query().Get("dry_run") == "true"

		res, err := connector.CancelSaleOrder(r.Context(), id, dryRun)
		if err != nil {
			RespondError(w, http.StatusBadRequest, err.Error())
			return
		}

		RespondJSON(w, http.StatusOK, res)
	}
}

func GetInvoices(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		connector, err := getConnector(r, cfg)
		if err != nil {
			RespondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		q := r.URL.Query()
		customerID := q.Get("customer_id")
		customerName := q.Get("customer_name")
		relationship := q.Get("relationship")
		orderID := q.Get("order_id")
		limit := queryInt(q.Get("limit"), 20, 100)
		offset := queryInt(q.Get("offset"), 0, 100000)

		if relationship != "" && relationship != "invoices_for_order" {
			RespondError(w, http.StatusBadRequest, "unsupported relationship: "+relationship)
			return
		}

		if customerID == "" && customerName != "" {
			id, candidates, err := resolveCustomerID(r.Context(), connector, customerName)
			if err != nil {
				RespondError(w, http.StatusBadGateway, "customer resolution failed: "+err.Error())
				return
			}
			if id == "" {
				status := http.StatusNotFound
				code := "ENTITY_NOT_FOUND"
				if len(candidates) > 1 {
					status = http.StatusConflict
					code = "AMBIGUOUS_ENTITY"
				}
				RespondJSON(w, status, map[string]any{
					"error":            code,
					"field":            "customer",
					"message":          fmt.Sprintf("could not resolve customer %q", customerName),
					"candidates":       candidates,
					"suggested_action": "choose_one",
				})
				return
			}
			customerID = id
		}

		status := firstNonEmpty(q.Get("state"), q.Get("status"))
		paymentState := q.Get("payment_state")
		// "Unpaid" means a posted invoice that is not settled; drafts are not
		// invoices yet. An explicit state still wins.
		if status == "" && paymentState != "" {
			status = "posted"
		}
		filter := domain.InvoiceFilter{
			CustomerID:   customerID,
			OrderID:      orderID,
			MoveType:     q.Get("move_type"),
			Status:       status,
			PaymentState: paymentState,
			Overdue:      queryBool(q.Get("overdue")),
			Limit:        limit,
			Offset:       offset,
		}
		invoices, err := connector.GetInvoices(r.Context(), filter)
		if err != nil {
			RespondError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := verifyInvoices(invoices, filter, time.Now()); err != nil {
			RespondError(w, http.StatusBadGateway, err.Error())
			return
		}

		RespondJSON(w, http.StatusOK, map[string]interface{}{
			"status": "success",
			"count":  len(invoices),
			"data":   invoices,
			"metadata": map[string]any{
				"limit":   limit,
				"offset":  offset,
				"overdue": filter.Overdue,
			},
		})
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func DeliverSaleOrder(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		connector, err := getConnector(r, cfg)
		if err != nil {
			RespondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		vars := mux.Vars(r)
		id := vars["id"]
		dryRun := r.URL.Query().Get("dry_run") == "true"

		res, err := connector.DeliverSaleOrder(r.Context(), id, dryRun)
		if err != nil {
			RespondError(w, http.StatusBadRequest, err.Error())
			return
		}

		RespondJSON(w, http.StatusOK, res)
	}
}

func CreateInvoice(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		connector, err := getConnector(r, cfg)
		if err != nil {
			RespondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		vars := mux.Vars(r)
		id := vars["id"]
		dryRun := r.URL.Query().Get("dry_run") == "true"

		res, err := connector.CreateInvoice(r.Context(), id, dryRun)
		if err != nil {
			RespondError(w, http.StatusBadRequest, err.Error())
			return
		}

		RespondJSON(w, http.StatusOK, res)
	}
}

func PostInvoice(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		connector, err := getConnector(r, cfg)
		if err != nil {
			RespondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		vars := mux.Vars(r)
		id := vars["id"]
		dryRun := r.URL.Query().Get("dry_run") == "true"

		res, err := connector.PostInvoice(r.Context(), id, dryRun)
		if err != nil {
			RespondError(w, http.StatusBadRequest, err.Error())
			return
		}

		RespondJSON(w, http.StatusOK, res)
	}
}
