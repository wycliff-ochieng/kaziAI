package handlers

import (
	"context"
	"net/http"

	"erpagent-backend/internal/config"
	"erpagent-backend/internal/domain"
)

// countHandler answers "how many X" questions with an exact adapter count.
func countHandler(cfg *config.Config, entity string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		connector, err := getConnector(r, cfg)
		if err != nil {
			RespondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		q := r.URL.Query()
		state := firstNonEmpty(q.Get("state"), q.Get("status"))
		paymentState := q.Get("payment_state")
		if entity == domain.CountEntityInvoices && state == "" && paymentState != "" {
			state = "posted"
		}
		filter := domain.CountFilter{
			CustomerID:   q.Get("customer_id"),
			Search:       q.Get("search"),
			State:        state,
			PaymentState: paymentState,
			Overdue:      queryBool(q.Get("overdue")),
		}

		if filter.CustomerID == "" && q.Get("customer_name") != "" {
			id, candidates, err := resolveCustomerID(r.Context(), connector, q.Get("customer_name"))
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
					"message":          "could not resolve customer",
					"candidates":       candidates,
					"suggested_action": "choose_one",
				})
				return
			}
			filter.CustomerID = id
		}

		count, exact := countExact(r.Context(), connector, entity, filter)
		RespondJSON(w, http.StatusOK, map[string]any{
			"status": "success",
			"count":  count,
			"metadata": map[string]any{
				"entity": entity,
				"exact":  exact,
			},
		})
	}
}

// countRecords prefers an exact adapter count and otherwise falls back to a
// bounded fetch, flagging the answer as inexact.
func countExact(ctx context.Context, connector domain.ERPConnector, entity string, filter domain.CountFilter) (int, bool) {
	if counter, ok := connector.(domain.RecordCounter); ok {
		if n, err := counter.CountRecords(ctx, entity, filter); err == nil {
			return n, true
		}
	}
	switch entity {
	case domain.CountEntityCustomers:
		if rows, err := connector.GetCustomers(ctx, domain.CustomerFilter{Search: filter.Search, Limit: 1000}); err == nil {
			return len(rows), false
		}
	case domain.CountEntityProducts:
		if rows, err := connector.GetProducts(ctx, domain.ProductFilter{Search: filter.Search, Limit: 1000}); err == nil {
			return len(rows), false
		}
	case domain.CountEntityOrders:
		if rows, err := connector.GetSaleOrders(ctx, domain.SaleOrderFilter{CustomerID: filter.CustomerID, State: filter.State, Search: filter.Search, Limit: 1000}); err == nil {
			return len(rows), false
		}
	case domain.CountEntityInvoices:
		if rows, err := connector.GetInvoices(ctx, domain.InvoiceFilter{CustomerID: filter.CustomerID, Status: filter.State, PaymentState: filter.PaymentState, Overdue: filter.Overdue, Limit: 1000}); err == nil {
			return len(rows), false
		}
	case domain.CountEntityVendors:
		if rows, err := connector.GetVendors(ctx, domain.VendorFilter{Search: filter.Search, Limit: 1000}); err == nil {
			return len(rows), false
		}
	}
	return 0, false
}

func CountCustomers(cfg *config.Config) http.HandlerFunc {
	return countHandler(cfg, domain.CountEntityCustomers)
}
func CountProducts(cfg *config.Config) http.HandlerFunc {
	return countHandler(cfg, domain.CountEntityProducts)
}
func CountSaleOrders(cfg *config.Config) http.HandlerFunc {
	return countHandler(cfg, domain.CountEntityOrders)
}
func CountInvoices(cfg *config.Config) http.HandlerFunc {
	return countHandler(cfg, domain.CountEntityInvoices)
}
func CountVendors(cfg *config.Config) http.HandlerFunc {
	return countHandler(cfg, domain.CountEntityVendors)
}
