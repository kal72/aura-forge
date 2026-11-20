package config_test

import (
	"os"
	"testing"

	"aura-forge/internal/config"
)

func TestConfigSSOEnabled(t *testing.T) {
	// Case 1: Both empty
	cfg := &config.Config{
		ESIClientID:     "",
		ESIClientSecret: "",
	}
	if cfg.IsSSOEnabled() {
		t.Errorf("expected SSO to be disabled when credentials are empty")
	}

	// Case 2: Only client ID set
	cfg = &config.Config{
		ESIClientID:     "my-client-id",
		ESIClientSecret: "",
	}
	if cfg.IsSSOEnabled() {
		t.Errorf("expected SSO to be disabled when secret is missing")
	}

	// Case 3: Both set
	cfg = &config.Config{
		ESIClientID:     "my-client-id",
		ESIClientSecret: "my-client-secret",
	}
	if !cfg.IsSSOEnabled() {
		t.Errorf("expected SSO to be enabled when both credentials are set")
	}
}

func TestConfigDefaults(t *testing.T) {
	// Clear any env overrides for the test
	_ = os.Unsetenv("PORT")
	_ = os.Unsetenv("DATABASE_URL")

	cfg := config.Load()
	if cfg.Port == "" {
		t.Errorf("expected default port to be non-empty")
	}
	if cfg.DatabaseURL == "" {
		t.Errorf("expected default database url to be non-empty")
	}
	if cfg.ESIUserAgent == "" {
		t.Errorf("expected default user agent to be non-empty")
	}
}
