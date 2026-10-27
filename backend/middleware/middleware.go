package middleware

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"erpagent-backend/internal/auth"
)

type contextKey string

const (
	TenantIDKey contextKey = "tenantID"
	UserIDKey   contextKey = "userID"
)

// Logger logs incoming HTTP requests with method, path, and duration.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s - %s %s [%v]", r.RemoteAddr, r.Method, r.URL.Path, time.Since(start))
	})
}

// Recover handles panics gracefully and returns a 500 Internal Server Error.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("Recovered from panic: %v", rec)
				http.Error(w, `{"error":"Internal Server Error"}`, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// TenantAuth extracts X-Tenant-ID header and injects it into the request context.
func TenantAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.Header.Get("X-Tenant-ID")
		if tenantID == "" {
			tenantID = "default_tenant"
		}
		ctx := context.WithValue(r.Context(), TenantIDKey, tenantID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Auth resolves the caller from a session cookie, an internal service token, or
// (outside production) the legacy X-Tenant-ID header. It injects tenant/user
// identity into the request context and rejects unauthenticated requests.
func Auth(store auth.Store, internalToken, environment string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID, userID, ok := resolveRequest(r, store, internalToken, environment, false)
			if !ok {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "authentication required"})
				return
			}
			ctx := context.WithValue(r.Context(), TenantIDKey, tenantID)
			ctx = context.WithValue(ctx, UserIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ResolveTenant resolves identity for the WebSocket handshake. Query fallback is
// allowed outside production so local tooling can connect without a cookie.
func ResolveTenant(r *http.Request, store auth.Store, internalToken, environment string) (tenantID, userID string, ok bool) {
	return resolveRequest(r, store, internalToken, environment, true)
}

func resolveRequest(r *http.Request, store auth.Store, internalToken, environment string, allowQuery bool) (string, string, bool) {
	if store != nil {
		if cookie, err := r.Cookie(auth.SessionCookieName); err == nil && cookie.Value != "" {
			if user, err := store.GetUserBySession(r.Context(), cookie.Value); err == nil && user != nil {
				return user.TenantID, user.ID, true
			}
		}
	}
	if internalToken != "" {
		provided := r.Header.Get("X-Internal-Token")
		if subtle.ConstantTimeCompare([]byte(provided), []byte(internalToken)) == 1 {
			tenantID := r.Header.Get("X-Tenant-ID")
			if tenantID == "" {
				tenantID = "default_tenant"
			}
			return tenantID, "service", true
		}
	}
	if environment != "production" {
		if tenantID := r.Header.Get("X-Tenant-ID"); tenantID != "" {
			return tenantID, "dev", true
		}
		if allowQuery {
			if tenantID := r.URL.Query().Get("tenant_id"); tenantID != "" {
				return tenantID, "dev", true
			}
		}
	}
	return "", "", false
}

// SetSessionCookie writes the HttpOnly session cookie.
func SetSessionCookie(w http.ResponseWriter, token string, secure bool, ttl time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(ttl.Seconds()),
	})
}

// ClearSessionCookie expires the session cookie.
func ClearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
