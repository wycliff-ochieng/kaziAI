package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
