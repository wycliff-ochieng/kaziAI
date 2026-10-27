package handlers

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"erpagent-backend/internal/auth"
	"erpagent-backend/internal/config"
	"erpagent-backend/internal/llmsettings"
)

type llmSettingsRequest struct {
	Provider  string `json:"provider"`
	BaseURL   string `json:"base_url"`
	Model     string `json:"model"`
	APIKey    string `json:"api_key"`
	MaxTokens int    `json:"max_tokens"`
}

func defaultLLMConfig(cfg *config.Config) llmsettings.Config {
	return llmsettings.Config{
		Provider:  cfg.LLMProvider,
		BaseURL:   cfg.LLMBaseURL,
		Model:     cfg.LLMModel,
		APIKey:    cfg.LLMAPIKey,
		MaxTokens: cfg.LLMMaxTokens,
	}.WithDefaults()
}

func currentLLMConfig(ctx context.Context, store llmsettings.Store, cfg *config.Config) llmsettings.Config {
	if store != nil {
		if saved, err := store.Get(ctx); err == nil && saved != nil {
			return saved.WithDefaults()
		}
	}
	return defaultLLMConfig(cfg)
}

func mergeLLMRequest(req llmSettingsRequest, base llmsettings.Config) llmsettings.Config {
	out := base
	if strings.TrimSpace(req.Provider) != "" {
		out.Provider = strings.TrimSpace(req.Provider)
	}
	if strings.TrimSpace(req.BaseURL) != "" {
		out.BaseURL = strings.TrimSpace(req.BaseURL)
	}
	if strings.TrimSpace(req.Model) != "" {
		out.Model = strings.TrimSpace(req.Model)
	}
	if req.MaxTokens > 0 {
		out.MaxTokens = req.MaxTokens
	}
	if req.APIKey != "" {
		out.APIKey = strings.TrimSpace(req.APIKey)
	}
	return out.WithDefaults()
}

func validateLLMConfig(c llmsettings.Config) error {
	if strings.TrimSpace(c.Model) == "" {
		return fmt.Errorf("model is required")
	}
	u, err := url.Parse(c.BaseURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("base_url must be an absolute URL, e.g. https://api.groq.com/openai/v1")
	}
	return nil
}

// settingsAdmin limits deployment-wide AI model settings to the configured
// administrator. In development an unset ADMIN_EMAIL leaves them open.
func settingsAdmin(r *http.Request, authStore auth.Store, cfg *config.Config) bool {
	if strings.TrimSpace(cfg.AdminEmail) == "" {
		return cfg.Environment != "production"
	}
	if authStore == nil {
		return false
	}
	userID := currentUserID(r)
	if userID == "" || userID == "service" || userID == "dev" {
		return false
	}
	user, err := authStore.GetUserByID(r.Context(), userID)
	return err == nil && strings.EqualFold(user.Email, cfg.AdminEmail)
}

func internalTokenOK(r *http.Request, cfg *config.Config) bool {
	expected := cfg.InternalAPIToken
	if expected == "" {
		// Local development fallback. Production must set INTERNAL_API_TOKEN.
		return true
	}
	got := r.Header.Get("X-Internal-Token")
	return subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1
}

// GetLLMSettings returns the saved runtime LLM configuration with the API key
// redacted.
func GetLLMSettings(store llmsettings.Store, authStore auth.Store, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !settingsAdmin(r, authStore, cfg) {
			RespondError(w, http.StatusForbidden, "Only the deployment administrator can view AI model settings")
			return
		}
		c := currentLLMConfig(r.Context(), store, cfg)
		RespondJSON(w, http.StatusOK, map[string]any{"status": "success", "data": c.Redacted()})
	}
}

// SaveLLMSettings persists the LLM configuration and pushes it to the AI
// engine. A failed push does not lose the saved config; it is reported as a
// warning so the operator can retry.
func SaveLLMSettings(store llmsettings.Store, authStore auth.Store, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !settingsAdmin(r, authStore, cfg) {
			RespondError(w, http.StatusForbidden, "Only the deployment administrator can change AI model settings")
			return
		}
		var req llmSettingsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			RespondError(w, http.StatusBadRequest, "Invalid LLM settings payload")
			return
		}

		base := currentLLMConfig(r.Context(), store, cfg)
		next := mergeLLMRequest(req, base)
		if err := validateLLMConfig(next); err != nil {
			RespondError(w, http.StatusBadRequest, err.Error())
			return
		}

		saved := next
		if store != nil {
			var err error
			saved, err = store.Save(r.Context(), next)
			if err != nil {
				RespondError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}

		applied := false
		warning := ""
		if err := pushLLMConfig(r.Context(), cfg, saved); err != nil {
			warning = "saved locally but AI engine did not apply it: " + err.Error()
		} else {
			applied = true
		}

		payload := map[string]any{
			"status":  "success",
			"data":    saved.Redacted(),
			"applied": applied,
		}
		if warning != "" {
			payload["warning"] = warning
		}
		RespondJSON(w, http.StatusOK, payload)
	}
}

// TestLLMSettings probes the provider through the AI engine. If the request
// omits a key, the saved key is used.
func TestLLMSettings(store llmsettings.Store, authStore auth.Store, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !settingsAdmin(r, authStore, cfg) {
			RespondError(w, http.StatusForbidden, "Only the deployment administrator can test AI model settings")
			return
		}
		var req llmSettingsRequest
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&req)
		}
		base := currentLLMConfig(r.Context(), store, cfg)
		candidate := mergeLLMRequest(req, base)

		result, err := callAIEngineAdmin(r.Context(), cfg, "/api/v1/admin/llm/test", candidate)
		if err != nil {
			RespondJSON(w, http.StatusOK, map[string]any{
				"ok":      false,
				"message": err.Error(),
			})
			return
		}
		RespondJSON(w, http.StatusOK, result)
	}
}

// InternalLLMConfig is consumed by the AI engine at startup. It is protected
// by a shared token, not tenant auth, because the AI engine is a single
// internal service.
func InternalLLMConfig(store llmsettings.Store, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !internalTokenOK(r, cfg) {
			RespondError(w, http.StatusUnauthorized, "invalid internal token")
			return
		}
		c := currentLLMConfig(r.Context(), store, cfg)
		RespondJSON(w, http.StatusOK, map[string]any{
			"provider":   c.Provider,
			"base_url":   c.BaseURL,
			"model":      c.Model,
			"api_key":    c.APIKey,
			"max_tokens": c.MaxTokens,
		})
	}
}

func pushLLMConfig(ctx context.Context, cfg *config.Config, c llmsettings.Config) error {
	_, err := callAIEngineAdmin(ctx, cfg, "/api/v1/admin/llm", c)
	return err
}

func callAIEngineAdmin(ctx context.Context, cfg *config.Config, path string, c llmsettings.Config) (map[string]any, error) {
	if strings.TrimSpace(cfg.AIEngineURL) == "" {
		return nil, fmt.Errorf("AI_ENGINE_URL is not configured")
	}
	body, err := json.Marshal(map[string]any{
		"provider":   c.Provider,
		"base_url":   c.BaseURL,
		"model":      c.Model,
		"api_key":    c.APIKey,
		"max_tokens": c.MaxTokens,
	})
	if err != nil {
		return nil, err
	}

	endpoint := strings.TrimSuffix(cfg.AIEngineURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.InternalAPIToken != "" {
		req.Header.Set("X-Internal-Token", cfg.InternalAPIToken)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("AI engine request failed: %w", err)
	}
	defer resp.Body.Close()

	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("AI engine returned invalid JSON: %w", err)
	}
	if resp.StatusCode >= 400 {
		msg, _ := payload["detail"].(string)
		if msg == "" {
			msg = fmt.Sprintf("AI engine returned HTTP %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("%s", msg)
	}
	return payload, nil
}
