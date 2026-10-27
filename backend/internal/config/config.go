package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port         string
	Environment  string
	AdminEmail   string
	DatabaseURL  string
	RedisURL     string
	AIEngineURL  string
	AIEngineGrPC string
	VaultAddr    string
	OdooURL      string
	OdooDB       string
	OdooUser     string
	OdooPassword string

	// Runtime-configurable LLM fallback values. The UI-managed values in
	// Postgres take precedence once saved.
	LLMProvider  string
	LLMBaseURL   string
	LLMModel     string
	LLMAPIKey    string
	LLMMaxTokens int

	// Shared secret for backend -> AI engine admin calls and for the AI
	// engine's startup config pull. Empty is allowed in local dev only.
	InternalAPIToken string
	// Key used to encrypt stored LLM API keys at rest.
	SettingsEncryptionKey string
}

func LoadConfig() *Config {
	return &Config{
		Port:         getEnv("PORT", "3000"),
		Environment:  getEnv("ENVIRONMENT", "development"),
		AdminEmail:   getEnv("ADMIN_EMAIL", ""),
		DatabaseURL:  getEnv("DATABASE_URL", "host=localhost user=postgres password=postgres dbname=erpagent port=5432 sslmode=disable"),
		RedisURL:     getEnv("REDIS_URL", "localhost:6379"),
		AIEngineURL:  getEnv("AI_ENGINE_URL", "http://localhost:8000"),
		AIEngineGrPC: getEnv("AI_ENGINE_GRPC", "localhost:50051"),
		VaultAddr:    getEnv("VAULT_ADDR", "http://localhost:8200"),
		OdooURL:      getEnv("ODOO_URL", ""),
		OdooDB:       getEnv("ODOO_DB", ""),
		OdooUser:     getEnv("ODOO_USER", ""),
		OdooPassword: getEnv("ODOO_PASSWORD", ""),

		LLMProvider:           getEnv("LLM_PROVIDER", "groq"),
		LLMBaseURL:            getEnv("LLM_BASE_URL", "https://api.groq.com/openai/v1"),
		LLMModel:              getEnv("LLM_MODEL", "llama3-70b-8192"),
		LLMAPIKey:             getEnv("LLM_API_KEY", ""),
		LLMMaxTokens:          getEnvInt("LLM_MAX_TOKENS", 400),
		InternalAPIToken:      getEnv("INTERNAL_API_TOKEN", ""),
		SettingsEncryptionKey: getEnv("SETTINGS_ENCRYPTION_KEY", ""),
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	value, exists := os.LookupEnv(key)
	if !exists || value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
