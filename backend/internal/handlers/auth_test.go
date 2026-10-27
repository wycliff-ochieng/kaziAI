package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"erpagent-backend/internal/auth"
	"erpagent-backend/internal/config"
	"erpagent-backend/middleware"
)

func doAuthJSON(t *testing.T, h http.Handler, method, path string, body any, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestRegisterLoginProfileAndPassword(t *testing.T) {
	store := auth.NewMemoryStore()
	cfg := &config.Config{Environment: "test"}

	rec := doAuthJSON(t, Register(store, cfg), http.MethodPost, "/api/v1/auth/register", map[string]string{
		"name": "Ada Lovelace", "email": "ADA@example.com", "password": "correct-horse",
	}, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register status = %d body=%s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Name != auth.SessionCookieName || cookies[0].Value == "" {
		t.Fatalf("expected session cookie, got %#v", cookies)
	}
	cookie := cookies[0]

	protected := middleware.Auth(store, "", cfg.Environment)(http.HandlerFunc(Me(store)))
	rec = doAuthJSON(t, protected, http.MethodGet, "/api/v1/auth/me", nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("me status = %d body=%s", rec.Code, rec.Body.String())
	}
	var me authResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &me); err != nil {
		t.Fatal(err)
	}
	if me.User.Email != "ada@example.com" || me.User.TenantID == "" {
		t.Fatalf("unexpected user: %#v", me.User)
	}

	rec = doAuthJSON(t, Login(store, cfg), http.MethodPost, "/api/v1/auth/login", map[string]string{
		"email": "ada@example.com", "password": "wrong-password",
	}, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password status = %d", rec.Code)
	}

	profile := middleware.Auth(store, "", cfg.Environment)(http.HandlerFunc(UpdateProfile(store)))
	rec = doAuthJSON(t, profile, http.MethodPatch, "/api/v1/auth/profile", map[string]string{
		"name": "Ada Byron", "email": "ada.byron@example.com",
	}, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("profile status = %d body=%s", rec.Code, rec.Body.String())
	}

	password := middleware.Auth(store, "", cfg.Environment)(http.HandlerFunc(ChangePassword(store)))
	rec = doAuthJSON(t, password, http.MethodPost, "/api/v1/auth/password", map[string]string{
		"current_password": "correct-horse", "new_password": "new-correct-horse",
	}, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("password status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = doAuthJSON(t, Login(store, cfg), http.MethodPost, "/api/v1/auth/login", map[string]string{
		"email": "ada.byron@example.com", "password": "new-correct-horse",
	}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("new password login status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRegisterRejectsWeakPasswordAndDuplicate(t *testing.T) {
	store := auth.NewMemoryStore()
	cfg := &config.Config{Environment: "test"}
	h := Register(store, cfg)

	rec := doAuthJSON(t, h, http.MethodPost, "/api/v1/auth/register", map[string]string{
		"name": "A", "email": "a@example.com", "password": "short",
	}, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("weak password status = %d", rec.Code)
	}

	body := map[string]string{"name": "A", "email": "a@example.com", "password": "long-enough"}
	if rec := doAuthJSON(t, h, http.MethodPost, "/api/v1/auth/register", body, nil); rec.Code != http.StatusCreated {
		t.Fatalf("first register status = %d body=%s", rec.Code, rec.Body.String())
	}
	if rec := doAuthJSON(t, h, http.MethodPost, "/api/v1/auth/register", body, nil); rec.Code != http.StatusConflict {
		t.Fatalf("duplicate register status = %d body=%s", rec.Code, rec.Body.String())
	}
}
