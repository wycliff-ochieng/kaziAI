package main

import (
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"

	"erpagent-backend/internal/audit"
	"erpagent-backend/internal/config"
	"erpagent-backend/internal/connectors"
	"erpagent-backend/internal/database"
	"erpagent-backend/internal/handlers"
	"erpagent-backend/internal/realtime"
	"erpagent-backend/middleware"
)

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Tenant-ID")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// @title ERPAgent Universal Gateway API
// @version 1.0
// @description Gorilla Mux API Gateway connecting frontend client prompts to AI Engine and ERP Connectors (Odoo, SAP, Dynamics).
// @host localhost:3000
// @BasePath /api/v1
func main() {
	cfg := config.LoadConfig()

	r := mux.NewRouter()

	// Global Middlewares
	r.Use(corsMiddleware)
	r.Use(middleware.Logger)
	r.Use(middleware.Recover)

	// API Subrouter (/api/v1)
	api := r.PathPrefix("/api/v1").Subrouter()

	// Public System Health
	api.HandleFunc("/health", handlers.HealthCheck).Methods(http.MethodGet)

	// Single multiplexed WebSocket: chat stream + tenant activity (locked Q7).
	// Tenant/conversation travel via query params (browsers can't set WS headers).
	var chatStore realtime.Store = realtime.NewMemoryStore()
	if rs, err := realtime.NewRedisStore(cfg.RedisURL); err != nil {
		log.Printf("Redis unavailable at %s (%v); using in-memory chat state", cfg.RedisURL, err)
	} else {
		chatStore = rs
	}

	// Shared Postgres pool for audit + connectors (locked Q8/Q9).
	// Falls back to in-memory stores when PG is unreachable (local dev).
	pg, pgErr := database.OpenPostgres(cfg)
	if pgErr != nil {
		log.Printf("Postgres unavailable (%v); using in-memory audit + connectors", pgErr)
	}

	// Immutable audit trail (locked Q8).
	var auditStore audit.Store = audit.NewMemoryStore()
	if pg != nil {
		if ps, err := audit.NewPostgresStore(pg); err != nil {
			log.Printf("Audit migrate failed (%v); using in-memory audit trail", err)
		} else {
			auditStore = ps
		}
	}

	// Tenant ERP connectors (locked Q9).
	var connStore connectors.Store = connectors.NewMemoryStore()
	if pg != nil {
		if cs, err := connectors.NewPostgresStore(pg); err != nil {
			log.Printf("Connectors migrate failed (%v); using in-memory connectors", err)
		} else {
			connStore = cs
		}
	}
	handlers.GlobalConnStore = connStore
	api.HandleFunc("/ws", handlers.ChatSocket(cfg, chatStore, auditStore, connStore))

	// Protected Subrouter with TenantAuth Middleware
	protected := api.PathPrefix("/").Subrouter()
	protected.Use(middleware.TenantAuth)

	// AI Intent & Natural Language Chat Endpoints
	protected.HandleFunc("/chat", handlers.HandleChat(cfg)).Methods(http.MethodPost)
	protected.HandleFunc("/intents", handlers.HandleChat(cfg)).Methods(http.MethodPost)

	// Sales App REST API Endpoints
	sales := protected.PathPrefix("/sales").Subrouter()
	sales.HandleFunc("/customers", handlers.GetCustomers(cfg)).Methods(http.MethodGet)
	sales.HandleFunc("/customers/count", handlers.CountCustomers(cfg)).Methods(http.MethodGet)
	sales.HandleFunc("/products", handlers.GetProducts(cfg)).Methods(http.MethodGet)
	sales.HandleFunc("/products/count", handlers.CountProducts(cfg)).Methods(http.MethodGet)
	sales.HandleFunc("/orders", handlers.GetSaleOrders(cfg)).Methods(http.MethodGet)
	sales.HandleFunc("/orders/count", handlers.CountSaleOrders(cfg)).Methods(http.MethodGet)
	sales.HandleFunc("/orders", handlers.CreateSaleOrder(cfg)).Methods(http.MethodPost)
	sales.HandleFunc("/orders/{id}", handlers.GetSaleOrderByID(cfg)).Methods(http.MethodGet)
	sales.HandleFunc("/orders/{id}/confirm", handlers.ConfirmSaleOrder(cfg)).Methods(http.MethodPost)
	sales.HandleFunc("/orders/{id}/cancel", handlers.CancelSaleOrder(cfg)).Methods(http.MethodPost)
	sales.HandleFunc("/orders/{id}/deliver", handlers.DeliverSaleOrder(cfg)).Methods(http.MethodPost)
	sales.HandleFunc("/orders/{id}/invoice", handlers.CreateInvoice(cfg)).Methods(http.MethodPost)
	sales.HandleFunc("/invoices", handlers.GetInvoices(cfg)).Methods(http.MethodGet)
	sales.HandleFunc("/invoices/count", handlers.CountInvoices(cfg)).Methods(http.MethodGet)
	sales.HandleFunc("/invoices/{id}/post", handlers.PostInvoice(cfg)).Methods(http.MethodPost)

	// Purchase App REST API Endpoints (canonical; ERP mapping lives in adapters)
	purchase := protected.PathPrefix("/purchase").Subrouter()
	purchase.HandleFunc("/vendors", handlers.GetVendors(cfg)).Methods(http.MethodGet)
	purchase.HandleFunc("/vendors/count", handlers.CountVendors(cfg)).Methods(http.MethodGet)
	purchase.HandleFunc("/orders", handlers.GetPurchaseOrders(cfg)).Methods(http.MethodGet)
	purchase.HandleFunc("/orders", handlers.CreateRFQ(cfg)).Methods(http.MethodPost)
	purchase.HandleFunc("/orders/{id}", handlers.GetPurchaseOrderByID(cfg)).Methods(http.MethodGet)
	purchase.HandleFunc("/orders/{id}/confirm", handlers.ConfirmPurchaseOrder(cfg)).Methods(http.MethodPost)
	purchase.HandleFunc("/orders/{id}/cancel", handlers.CancelPurchaseOrder(cfg)).Methods(http.MethodPost)
	purchase.HandleFunc("/orders/{id}/receive", handlers.ReceivePurchaseOrder(cfg)).Methods(http.MethodPost)
	purchase.HandleFunc("/orders/{id}/bill", handlers.CreateVendorBill(cfg)).Methods(http.MethodPost)
	purchase.HandleFunc("/bills", handlers.GetVendorBills(cfg)).Methods(http.MethodGet)
	purchase.HandleFunc("/bills/{id}/post", handlers.PostVendorBill(cfg)).Methods(http.MethodPost)

	// Immutable audit trail (locked Q8)
	protected.HandleFunc("/audit", handlers.GetAuditEvents(auditStore, cfg)).Methods(http.MethodGet)
	protected.HandleFunc("/audit/export", handlers.ExportAuditCSV(auditStore, cfg)).Methods(http.MethodGet)
	protected.HandleFunc("/audit/verify", handlers.VerifyAuditChain(auditStore, cfg)).Methods(http.MethodGet)

	// Tenant ERP connectors (locked Q9)
	protected.HandleFunc("/connectors", handlers.ListConnectors(connStore, cfg)).Methods(http.MethodGet)
	protected.HandleFunc("/connectors", handlers.UpsertConnector(connStore, cfg)).Methods(http.MethodPost)
	protected.HandleFunc("/connectors/test", handlers.TestConnectorAdhoc(connStore, cfg)).Methods(http.MethodPost)
	protected.HandleFunc("/connectors/{name}", handlers.DeleteConnector(connStore, cfg)).Methods(http.MethodDelete)
	protected.HandleFunc("/connectors/{name}/test", handlers.TestConnectorStored(connStore, cfg)).Methods(http.MethodPost)

	srv := &http.Server{
		Handler:      r,
		Addr:         ":" + cfg.Port,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Printf("Starting ERPAgent Gateway (Gorilla Mux) on port %s", cfg.Port)
	log.Fatal(srv.ListenAndServe())
}
