package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"erpagent-backend/internal/config"
	"erpagent-backend/internal/domain"
)

func GetVendors(cfg *config.Config) http.HandlerFunc {
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

		vendors, err := connector.GetVendors(r.Context(), domain.VendorFilter{
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
			"count":  len(vendors),
			"data":   vendors,
		})
	}
}

func GetPurchaseOrders(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		connector, err := getConnector(r, cfg)
		if err != nil {
			RespondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		vendorID := r.URL.Query().Get("vendor_id")
		state := r.URL.Query().Get("state")
		search := r.URL.Query().Get("search")

		orders, err := connector.GetPurchaseOrders(r.Context(), domain.PurchaseOrderFilter{
			VendorID: vendorID,
			State:    state,
			Search:   search,
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

func GetPurchaseOrderByID(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		connector, err := getConnector(r, cfg)
		if err != nil {
			RespondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		vars := mux.Vars(r)
		id := vars["id"]

		order, err := connector.GetPurchaseOrderByID(r.Context(), id)
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

func CreateRFQ(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		connector, err := getConnector(r, cfg)
		if err != nil {
			RespondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		var req domain.CreatePurchaseOrderRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			RespondError(w, http.StatusBadRequest, "Invalid RFQ payload")
			return
		}

		res, err := connector.CreateRFQ(r.Context(), req)
		if err != nil {
			RespondError(w, http.StatusBadRequest, err.Error())
			return
		}

		RespondJSON(w, http.StatusOK, res)
	}
}

func ConfirmPurchaseOrder(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		connector, err := getConnector(r, cfg)
		if err != nil {
			RespondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		vars := mux.Vars(r)
		id := vars["id"]
		dryRun := r.URL.Query().Get("dry_run") == "true"

		res, err := connector.ConfirmPurchaseOrder(r.Context(), id, dryRun)
		if err != nil {
			RespondError(w, http.StatusBadRequest, err.Error())
			return
		}

		RespondJSON(w, http.StatusOK, res)
	}
}

func CancelPurchaseOrder(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		connector, err := getConnector(r, cfg)
		if err != nil {
			RespondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		vars := mux.Vars(r)
		id := vars["id"]
		dryRun := r.URL.Query().Get("dry_run") == "true"

		res, err := connector.CancelPurchaseOrder(r.Context(), id, dryRun)
		if err != nil {
			RespondError(w, http.StatusBadRequest, err.Error())
			return
		}

		RespondJSON(w, http.StatusOK, res)
	}
}

func ReceivePurchaseOrder(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		connector, err := getConnector(r, cfg)
		if err != nil {
			RespondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		vars := mux.Vars(r)
		id := vars["id"]
		dryRun := r.URL.Query().Get("dry_run") == "true"

		res, err := connector.ReceivePurchaseOrder(r.Context(), id, dryRun)
		if err != nil {
			RespondError(w, http.StatusBadRequest, err.Error())
			return
		}

		RespondJSON(w, http.StatusOK, res)
	}
}

func GetVendorBills(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		connector, err := getConnector(r, cfg)
		if err != nil {
			RespondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		bills, err := connector.GetVendorBills(r.Context(), domain.VendorBillFilter{})
		if err != nil {
			RespondError(w, http.StatusBadRequest, err.Error())
			return
		}

		RespondJSON(w, http.StatusOK, map[string]interface{}{
			"status": "success",
			"count":  len(bills),
			"data":   bills,
		})
	}
}

func CreateVendorBill(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		connector, err := getConnector(r, cfg)
		if err != nil {
			RespondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		vars := mux.Vars(r)
		id := vars["id"]
		dryRun := r.URL.Query().Get("dry_run") == "true"

		res, err := connector.CreateVendorBill(r.Context(), id, dryRun)
		if err != nil {
			RespondError(w, http.StatusBadRequest, err.Error())
			return
		}

		RespondJSON(w, http.StatusOK, res)
	}
}

func PostVendorBill(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		connector, err := getConnector(r, cfg)
		if err != nil {
			RespondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		vars := mux.Vars(r)
		id := vars["id"]
		dryRun := r.URL.Query().Get("dry_run") == "true"

		res, err := connector.PostVendorBill(r.Context(), id, dryRun)
		if err != nil {
			RespondError(w, http.StatusBadRequest, err.Error())
			return
		}

		RespondJSON(w, http.StatusOK, res)
	}
}
