package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"erpagent-backend/internal/auth"
	"erpagent-backend/internal/config"
	"erpagent-backend/internal/llmsettings"
	"erpagent-backend/middleware"
)

func TestLLMSettingsRequireConfiguredAdmin(t *testing.T) {
	authStore := auth.NewMemoryStore()
	admin := auth.User{ID: "usr_admin", TenantID: "tenant_admin", Email: "admin@example.com", Name: "Admin"}
	other := auth.User{ID: "usr_other", TenantID: "tenant_other", Email: "other@example.com", Name: "Other"}
	for _, u := range []auth.User{admin, other} {
		if err := authStore.CreateUser(context.Background(), u, "hash"); err != nil {
			t.Fatal(err)
		}
	}
	adminToken, err := authStore.CreateSession(context.Background(), admin.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	otherToken, err := authStore.CreateSession(context.Background(), other.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{Environment: "production", AdminEmail: "admin@example.com"}
	handler := middleware.Auth(authStore, "", cfg.Environment)(
		GetLLMSettings(llmsettings.NewMemoryStore(), authStore, cfg),
	)

	for name, token := range map[string]string{"admin": adminToken, "other": otherToken} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/settings/llm", nil)
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if name == "admin" && rec.Code != http.StatusOK {
			t.Fatalf("admin: expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if name == "other" && rec.Code != http.StatusForbidden {
			t.Fatalf("other: expected 403, got %d: %s", rec.Code, rec.Body.String())
		}
	}
}
