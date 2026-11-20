package postgres_test

import (
	"strings"
	"testing"

	"aura-forge/internal/store/postgres"
)

func TestLoadEmbeddedMigrations_ValidPairsAndRules(t *testing.T) {
	migrations, err := postgres.LoadEmbeddedMigrations()
	if err != nil {
		t.Fatalf("LoadEmbeddedMigrations failed: %v", err)
	}
	if len(migrations) == 0 {
		t.Fatal("expected at least 1 embedded migration (001_init)")
	}
	if migrations[0].Version != "001_init" {
		t.Errorf("expected first migration to be 001_init, got %q", migrations[0].Version)
	}

	for _, m := range migrations {
		if strings.TrimSpace(m.UpSQL) == "" {
			t.Errorf("migration %s has empty UpSQL", m.Version)
		}
		if strings.TrimSpace(m.DownSQL) == "" {
			t.Errorf("migration %s has empty DownSQL", m.Version)
		}
		upperUp := strings.ToUpper(m.UpSQL)
		if strings.Contains(upperUp, "DOUBLE PRECISION") || strings.Contains(upperUp, " FLOAT") {
			t.Errorf("migration %s violates ISK precision rule: use NUMERIC(20,2) instead of FLOAT/DOUBLE PRECISION", m.Version)
		}
	}
}
