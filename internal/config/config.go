package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// Config holds all application configuration settings.
type Config struct {
	Port             string
	DatabaseURL      string
	SessionSecret    string
	AdminUsername    string
	AdminPassword    string
	ESIUserAgent     string
	ESIClientID      string
	ESIClientSecret  string
	ESICallbackURL   string
	AIBaseURL        string
	AIAPIKey         string
	AIModel          string
	AITimeoutSeconds int
}

// Load loads the configuration from environment variables, optionally reading from .env file if present.
func Load() *Config {
	loadDotEnv(".env")

	aiTimeout := 60
	if raw := strings.TrimSpace(getEnv("AI_TIMEOUT_SECONDS", "60")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 5 && parsed <= 300 {
			aiTimeout = parsed
		}
	}

	cfg := &Config{
		Port:             getEnv("PORT", "8080"),
		DatabaseURL:      getEnv("DATABASE_URL", "postgres://postgres:postgres_password@localhost:5432/aura_forge?sslmode=disable"),
		SessionSecret:    getEnv("SESSION_SECRET", "aura-forge-default-secret-key-32bytes"),
		AdminUsername:    strings.TrimSpace(getEnv("ADMIN_USERNAME", "admin")),
		AdminPassword:    getEnv("ADMIN_PASSWORD", "admin123"),
		ESIUserAgent:     getEnv("ESI_USER_AGENT", "aura-forge/1.0 (EVE Market & Industry Copilot; contact: pilot@new-eden.org)"),
		ESIClientID:      strings.TrimSpace(getEnv("ESI_CLIENT_ID", "")),
		ESIClientSecret:  strings.TrimSpace(getEnv("ESI_CLIENT_SECRET", "")),
		ESICallbackURL:   getEnv("ESI_CALLBACK_URL", "http://localhost:8080/auth/callback"),
		AIBaseURL:        strings.TrimRight(strings.TrimSpace(getEnv("AI_BASE_URL", "https://api.openai.com/v1")), "/"),
		AIAPIKey:         strings.TrimSpace(getEnv("AI_API_KEY", "")),
		AIModel:          strings.TrimSpace(getEnv("AI_MODEL", "gpt-4.1-mini")),
		AITimeoutSeconds: aiTimeout,
	}

	return cfg
}

// IsSSOEnabled returns true if both ESI_CLIENT_ID and ESI_CLIENT_SECRET are configured.
func (c *Config) IsSSOEnabled() bool {
	return c.ESIClientID != "" && c.ESIClientSecret != ""
}

// IsAIEnabled returns true if AI_API_KEY and AI_BASE_URL are configured.
func (c *Config) IsAIEnabled() bool {
	return c.AIAPIKey != "" && c.AIBaseURL != "" && c.AIModel != ""
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && strings.TrimSpace(val) != "" {
		return val
	}
	return defaultVal
}

// loadDotEnv reads key=value pairs from a .env file and sets them in the environment if not already set.
func loadDotEnv(filepath string) {
	file, err := os.Open(filepath)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, `"'`)

		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, val)
		}
	}
}
