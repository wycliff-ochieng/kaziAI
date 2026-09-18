package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"erpagent-backend/internal/config"
	"erpagent-backend/internal/domain"
	"erpagent-backend/internal/factory"
	"erpagent-backend/middleware"
)

func getConnector(r *http.Request, cfg *config.Config) (domain.ERPConnector, error) {
	tenantID, _ := r.Context().Value(middleware.TenantIDKey).(string)
	if tenantID == "" {
		tenantID = "default_tenant"
	}
	erpType := r.URL.Query().Get("erp_type")

	tenantCfg := factory.TenantConfig{
		TenantID: tenantID,
		ERPType:  erpType,
		BaseURL:  cfg.OdooURL,
		DBName:   cfg.OdooDB,
		Username: cfg.OdooUser,
		Password: cfg.OdooPassword,
	}

	return factory.GetERPConnector(tenantCfg)
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

		customerID := r.URL.Query().Get("customer_id")
		state := r.URL.Query().Get("state")
		search := r.URL.Query().Get("search")

		orders, err := connector.GetSaleOrders(r.Context(), domain.SaleOrderFilter{
			CustomerID: customerID,
			State:      state,
			Search:     search,
		})
		if err != nil {
			RespondError(w, http.StatusBadRequest, err.Error())
			return
		}

		RespondJSON(w, http.StatusOK, map[string]interface{}{
			"status": "success",
			"count":  len(orders),
			"data":   orders,
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

		invoices, err := connector.GetInvoices(r.Context(), domain.InvoiceFilter{})
		if err != nil {
			RespondError(w, http.StatusBadRequest, err.Error())
			return
		}

		RespondJSON(w, http.StatusOK, map[string]interface{}{
			"status": "success",
			"count":  len(invoices),
			"data":   invoices,
		})
	}
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
