package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"erpagent-backend/internal/auth"
	"erpagent-backend/internal/config"
	"erpagent-backend/middleware"
)

const sessionTTL = 7 * 24 * time.Hour

type authResponse struct {
	Status string    `json:"status"`
	User   auth.User `json:"user"`
}

func authCookieSecure(r *http.Request, cfg *config.Config) bool {
	return cfg.Environment == "production" || r.TLS != nil
}

func currentUserID(r *http.Request) string {
	id, _ := r.Context().Value(middleware.UserIDKey).(string)
	return id
}

// Register creates a tenant-scoped account and starts a session.
func Register(store auth.Store, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name     string `json:"name"`
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			RespondError(w, http.StatusBadRequest, "Invalid registration payload")
			return
		}
		req.Name = strings.TrimSpace(req.Name)
		req.Email = auth.NormalizeEmail(req.Email)
		if req.Name == "" {
			RespondError(w, http.StatusBadRequest, "Name is required")
			return
		}
		if !auth.ValidEmail(req.Email) {
			RespondError(w, http.StatusBadRequest, "A valid email is required")
			return
		}
		if !auth.ValidPassword(req.Password) {
			RespondError(w, http.StatusBadRequest, "Password must be at least 8 characters")
			return
		}

		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			RespondError(w, http.StatusInternalServerError, "Could not secure password")
			return
		}
		user := auth.User{
			ID:       auth.NewID("usr"),
			TenantID: auth.NewID("tenant"),
			Email:    req.Email,
			Name:     req.Name,
		}
		if err := store.CreateUser(r.Context(), user, hash); err != nil {
			if err == auth.ErrEmailTaken {
				RespondError(w, http.StatusConflict, "An account with that email already exists")
				return
			}
			RespondError(w, http.StatusInternalServerError, "Could not create account")
			return
		}
		if created, err := store.GetUserByID(r.Context(), user.ID); err == nil {
			user = *created
		}

		token, err := store.CreateSession(r.Context(), user.ID, sessionTTL)
		if err != nil {
			RespondError(w, http.StatusInternalServerError, "Account created but sign-in failed")
			return
		}
		middleware.SetSessionCookie(w, token, authCookieSecure(r, cfg), sessionTTL)
		RespondJSON(w, http.StatusCreated, authResponse{Status: "success", User: user})
	}
}

// Login verifies credentials and starts a session.
func Login(store auth.Store, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			RespondError(w, http.StatusBadRequest, "Invalid login payload")
			return
		}
		acct, err := store.GetAccountByEmail(r.Context(), auth.NormalizeEmail(req.Email))
		if err != nil || acct == nil || !auth.CheckPassword(acct.PasswordHash, req.Password) {
			RespondError(w, http.StatusUnauthorized, "Invalid email or password")
			return
		}

		token, err := store.CreateSession(r.Context(), acct.ID, sessionTTL)
		if err != nil {
			RespondError(w, http.StatusInternalServerError, "Could not start session")
			return
		}
		middleware.SetSessionCookie(w, token, authCookieSecure(r, cfg), sessionTTL)
		RespondJSON(w, http.StatusOK, authResponse{Status: "success", User: acct.User})
	}
}

// Logout revokes the current session and clears the cookie.
func Logout(store auth.Store, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie(auth.SessionCookieName); err == nil && cookie.Value != "" {
			_ = store.DeleteSession(r.Context(), cookie.Value)
		}
		middleware.ClearSessionCookie(w, authCookieSecure(r, cfg))
		RespondJSON(w, http.StatusOK, map[string]string{"status": "success"})
	}
}

// Me returns the authenticated account.
func Me(store auth.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := currentUserID(r)
		if userID == "" || userID == "service" || userID == "dev" {
			RespondError(w, http.StatusUnauthorized, "A signed-in account is required")
			return
		}
		user, err := store.GetUserByID(r.Context(), userID)
		if err != nil {
			RespondError(w, http.StatusUnauthorized, "Session is no longer valid")
			return
		}
		RespondJSON(w, http.StatusOK, authResponse{Status: "success", User: *user})
	}
}

// UpdateProfile updates the signed-in user's name and email.
func UpdateProfile(store auth.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := currentUserID(r)
		if userID == "" || userID == "service" || userID == "dev" {
			RespondError(w, http.StatusUnauthorized, "A signed-in account is required")
			return
		}
		current, err := store.GetUserByID(r.Context(), userID)
		if err != nil {
			RespondError(w, http.StatusUnauthorized, "Session is no longer valid")
			return
		}
		var req struct {
			Name  *string `json:"name"`
			Email *string `json:"email"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			RespondError(w, http.StatusBadRequest, "Invalid profile payload")
			return
		}
		name := current.Name
		email := current.Email
		if req.Name != nil {
			name = strings.TrimSpace(*req.Name)
		}
		if req.Email != nil {
			email = auth.NormalizeEmail(*req.Email)
		}
		if name == "" {
			RespondError(w, http.StatusBadRequest, "Name cannot be empty")
			return
		}
		if !auth.ValidEmail(email) {
			RespondError(w, http.StatusBadRequest, "A valid email is required")
			return
		}
		updated, err := store.UpdateProfile(r.Context(), userID, name, email)
		if err != nil {
			if err == auth.ErrEmailTaken {
				RespondError(w, http.StatusConflict, "That email is already in use")
				return
			}
			RespondError(w, http.StatusInternalServerError, "Could not update profile")
			return
		}
		RespondJSON(w, http.StatusOK, authResponse{Status: "success", User: *updated})
	}
}

// ChangePassword verifies the current password and replaces the hash.
func ChangePassword(store auth.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := currentUserID(r)
		if userID == "" || userID == "service" || userID == "dev" {
			RespondError(w, http.StatusUnauthorized, "A signed-in account is required")
			return
		}
		var req struct {
			CurrentPassword string `json:"current_password"`
			NewPassword     string `json:"new_password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			RespondError(w, http.StatusBadRequest, "Invalid password payload")
			return
		}
		if !auth.ValidPassword(req.NewPassword) {
			RespondError(w, http.StatusBadRequest, "New password must be at least 8 characters")
			return
		}
		acct, err := store.GetAccountByID(r.Context(), userID)
		if err != nil {
			RespondError(w, http.StatusUnauthorized, "Session is no longer valid")
			return
		}
		if !auth.CheckPassword(acct.PasswordHash, req.CurrentPassword) {
			RespondError(w, http.StatusUnauthorized, "Current password is incorrect")
			return
		}
		hash, err := auth.HashPassword(req.NewPassword)
		if err != nil {
			RespondError(w, http.StatusInternalServerError, "Could not secure password")
			return
		}
		if err := store.UpdatePassword(r.Context(), userID, hash); err != nil {
			RespondError(w, http.StatusInternalServerError, "Could not update password")
			return
		}
		RespondJSON(w, http.StatusOK, map[string]string{"status": "success"})
	}
}
