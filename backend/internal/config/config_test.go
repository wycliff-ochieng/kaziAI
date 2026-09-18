package config

import (
	"os"
	"testing"
)

func TestLoadConfigAppliesSafeDefaults(t *testing.T) {
	os.Unsetenv("PORT")
	os.Unsetenv("AI_ENGINE_URL")
	cfg := LoadConfig()
	if cfg.Port != "3000" {
		t.Errorf("expected default port 3000, got %s", cfg.Port)
	}
	if cfg.AIEngineURL != "http://localhost:8000" {
		t.Errorf("expected default AI engine URL, got %s", cfg.AIEngineURL)
	}
}

func TestLoadConfigHonorsEnvironment(t *testing.T) {
	t.Setenv("PORT", "4001")
	t.Setenv("AI_ENGINE_URL", "http://ai:8000")
	cfg := LoadConfig()
	if cfg.Port != "4001" {
		t.Errorf("expected 4001, got %s", cfg.Port)
	}
	if cfg.AIEngineURL != "http://ai:8000" {
		t.Errorf("expected custom AI URL, got %s", cfg.AIEngineURL)
	}
}
