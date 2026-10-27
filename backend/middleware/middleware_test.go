package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"erpagent-backend/internal/auth"
)

func TestTenantAuthDefaultsToDefaultTenant(t *testing.T) {
	var got string
	next := TenantAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = r.Context().Value(TenantIDKey).(string)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	next.ServeHTTP(rec, req)

	if got != "default_tenant" {
		t.Errorf("expected default_tenant, got %q", got)
	}
}

func TestTenantAuthPropagatesProvidedTenant(t *testing.T) {
	var got string
	next := TenantAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = r.Context().Value(TenantIDKey).(string)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Tenant-ID", "azure")
	rec := httptest.NewRecorder()
	next.ServeHTTP(rec, req)

	if got != "azure" {
		t.Errorf("expected azure tenant, got %q", got)
	}
}

func TestRecoverConvertsPanicTo500(t *testing.T) {
	next := Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	next.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 after panic, got %d", rec.Code)
	}
}

func TestAuthRejectsTenantHeaderInProduction(t *testing.T) {
	store := auth.NewMemoryStore()
	next := Auth(store, "", "production")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler must not run without a session")
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Tenant-ID", "attacker")
	rec := httptest.NewRecorder()
	next.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAuthAcceptsSessionCookie(t *testing.T) {
	store := auth.NewMemoryStore()
	user := auth.User{ID: "usr_1", TenantID: "tenant_1", Email: "a@example.com", Name: "A"}
	if err := store.CreateUser(context.Background(), user, "hash"); err != nil {
		t.Fatal(err)
	}
	token, err := store.CreateSession(context.Background(), user.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	var tenant, userID string
	next := Auth(store, "", "production")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenant, _ = r.Context().Value(TenantIDKey).(string)
		userID, _ = r.Context().Value(UserIDKey).(string)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	rec := httptest.NewRecorder()
	next.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || tenant != "tenant_1" || userID != "usr_1" {
		t.Fatalf("status=%d tenant=%q user=%q", rec.Code, tenant, userID)
	}
}

func TestAuthAcceptsInternalServiceToken(t *testing.T) {
	store := auth.NewMemoryStore()
	var tenant, userID string
	next := Auth(store, "shared-secret", "production")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenant, _ = r.Context().Value(TenantIDKey).(string)
		userID, _ = r.Context().Value(UserIDKey).(string)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Internal-Token", "shared-secret")
	req.Header.Set("X-Tenant-ID", "tenant_service")
	rec := httptest.NewRecorder()
	next.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || tenant != "tenant_service" || userID != "service" {
		t.Fatalf("status=%d tenant=%q user=%q", rec.Code, tenant, userID)
	}
}

func TestAuthRejectsBadInternalServiceToken(t *testing.T) {
	store := auth.NewMemoryStore()
	next := Auth(store, "shared-secret", "production")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler must not run with a bad service token")
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Internal-Token", "wrong")
	req.Header.Set("X-Tenant-ID", "tenant_service")
	rec := httptest.NewRecorder()
	next.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}
