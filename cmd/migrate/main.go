package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"aura-forge/internal/config"
	"aura-forge/internal/store/postgres"
)

const migrationsDir = "migrations"

var nonAlphaNum = regexp.MustCompile(`[^a-z0-9]+`)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := strings.ToLower(strings.TrimSpace(os.Args[1]))

	if cmd == "create" {
		if len(os.Args) < 3 {
			log.Fatal("Usage: go run ./cmd/migrate create <migration_name>")
		}
		if err := createMigrationFiles(os.Args[2]); err != nil {
			log.Fatalf("create migration failed: %v", err)
		}
		return
	}

	cfg := config.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := postgres.NewPoolOnly(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer db.Close()

	switch cmd {
	case "up":
		count, err := db.MigrateUp(ctx)
		if err != nil {
			log.Fatalf("migrate up failed: %v", err)
		}
		if count == 0 {
			fmt.Println("No pending migrations. Database is up to date.")
		} else {
			fmt.Printf("Successfully applied %d migration(s).\n", count)
		}

	case "down":
		version, err := db.MigrateDown(ctx)
		if err != nil {
			log.Fatalf("migrate down failed: %v", err)
		}
		if version == "" {
			fmt.Println("No applied migrations to roll back.")
		} else {
			fmt.Printf("Successfully rolled back migration: %s\n", version)
		}

	case "status":
		rows, err := db.MigrationStatus(ctx)
		if err != nil {
			log.Fatalf("migrate status failed: %v", err)
		}
		fmt.Printf("%-35s %-10s %s\n", "VERSION", "STATUS", "APPLIED_AT")
		fmt.Println(strings.Repeat("-", 72))
		for _, r := range rows {
			status := "PENDING"
			appliedAt := "-"
			if r.Applied {
				status = "APPLIED"
				if r.AppliedAt != nil {
					appliedAt = r.AppliedAt.Format(time.RFC3339)
				}
			}
			fmt.Printf("%-35s %-10s %s\n", r.Version, status, appliedAt)
		}

	default:
		printUsage()
		os.Exit(1)
	}
}

func createMigrationFiles(rawName string) error {
	slug := strings.Trim(nonAlphaNum.ReplaceAllString(strings.ToLower(strings.TrimSpace(rawName)), "_"), "_")
	if slug == "" {
		return fmt.Errorf("invalid migration name %q", rawName)
	}

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("reading %s: %w", migrationsDir, err)
	}

	maxNum := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		parts := strings.SplitN(e.Name(), "_", 2)
		if len(parts) < 2 {
			continue
		}
		if n, err := strconv.Atoi(parts[0]); err == nil && n > maxNum {
			maxNum = n
		}
	}

	nextPrefix := fmt.Sprintf("%03d_%s", maxNum+1, slug)
	upPath := filepath.Join(migrationsDir, nextPrefix+".up.sql")
	downPath := filepath.Join(migrationsDir, nextPrefix+".down.sql")

	upTemplate := fmt.Sprintf("-- %s.up.sql\n-- Aturan: Gunakan NUMERIC(20,2) untuk ISK, TIMESTAMPTZ untuk waktu, dan IF NOT EXISTS.\n\n", nextPrefix)
	downTemplate := fmt.Sprintf("-- %s.down.sql\n-- Rollback perubahan dari %s.up.sql secara aman (IF EXISTS).\n\n", nextPrefix, nextPrefix)

	if err := os.WriteFile(upPath, []byte(upTemplate), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(downPath, []byte(downTemplate), 0o644); err != nil {
		return err
	}

	fmt.Printf("Created migration pair:\n  + %s\n  + %s\n", upPath, downPath)
	return nil
}

func printUsage() {
	fmt.Println("Usage: go run ./cmd/migrate <command>")
	fmt.Println("Commands:")
	fmt.Println("  up             Apply all pending SQL migrations")
	fmt.Println("  down           Rollback the last applied SQL migration")
	fmt.Println("  status         Show status of all SQL migrations")
	fmt.Println("  create <name>  Generate next NNN_<name>.up.sql & .down.sql pair")
}
