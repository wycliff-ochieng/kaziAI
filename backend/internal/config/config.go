package config

import (
	"os"
)

type Config struct {
	Port         string
	DatabaseURL  string
	RedisURL     string
	AIEngineURL  string
	AIEngineGrPC string
	VaultAddr    string
	OdooURL      string
	OdooDB       string
	OdooUser     string
	OdooPassword string
}

func LoadConfig() *Config {
	return &Config{
		Port:         getEnv("PORT", "3000"),
		DatabaseURL:  getEnv("DATABASE_URL", "host=localhost user=postgres password=postgres dbname=erpagent port=5432 sslmode=disable"),
		RedisURL:     getEnv("REDIS_URL", "localhost:6379"),
		AIEngineURL:  getEnv("AI_ENGINE_URL", "http://localhost:8000"),
		AIEngineGrPC: getEnv("AI_ENGINE_GRPC", "localhost:50051"),
		VaultAddr:    getEnv("VAULT_ADDR", "http://localhost:8200"),
		OdooURL:      getEnv("ODOO_URL", ""),
		OdooDB:       getEnv("ODOO_DB", ""),
		OdooUser:     getEnv("ODOO_USER", ""),
		OdooPassword: getEnv("ODOO_PASSWORD", ""),
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
