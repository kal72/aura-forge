package postgres

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"aura-forge/migrations"
)

// MigrationFile represents a versioned up/down SQL migration pair.
type MigrationFile struct {
	Version string
	UpSQL   string
	DownSQL string
}

// MigrationStatusRow reports whether a migration version has been applied.
type MigrationStatusRow struct {
	Version   string
	Applied   bool
	AppliedAt *time.Time
}

// LoadEmbeddedMigrations reads, pairs, and validates all *.up.sql / *.down.sql files embedded in root migrations/.
func LoadEmbeddedMigrations() ([]MigrationFile, error) {
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("reading embedded migrations dir: %w", err)
	}

	byVersion := make(map[string]*MigrationFile)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}

		contentBytes, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			return nil, fmt.Errorf("reading migration %s: %w", name, err)
		}
		content := string(contentBytes)

		switch {
		case strings.HasSuffix(name, ".up.sql"):
			version := strings.TrimSuffix(name, ".up.sql")
			m := getOrCreateMigration(byVersion, version)
			m.UpSQL = content
		case strings.HasSuffix(name, ".down.sql"):
			version := strings.TrimSuffix(name, ".down.sql")
			m := getOrCreateMigration(byVersion, version)
			m.DownSQL = content
		default:
			return nil, fmt.Errorf("invalid migration filename %q: must end with .up.sql or .down.sql", name)
		}
	}

	migrations := make([]MigrationFile, 0, len(byVersion))
	for _, m := range byVersion {
		if strings.TrimSpace(m.UpSQL) == "" {
			return nil, fmt.Errorf("migration %q is missing .up.sql content", m.Version)
		}
		if strings.TrimSpace(m.DownSQL) == "" {
			return nil, fmt.Errorf("migration %q is missing .down.sql content", m.Version)
		}
		migrations = append(migrations, *m)
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

func getOrCreateMigration(m map[string]*MigrationFile, version string) *MigrationFile {
	if existing, ok := m[version]; ok {
		return existing
	}
	entry := &MigrationFile{Version: version}
	m[version] = entry
	return entry
}

func (d *DB) ensureSchemaMigrationsTable(ctx context.Context) error {
	const ddl = `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`
	_, err := d.Pool.Exec(ctx, ddl)
	if err != nil {
		return fmt.Errorf("ensuring schema_migrations table: %w", err)
	}
	return nil
}

func (d *DB) getAppliedMigrations(ctx context.Context) (map[string]time.Time, error) {
	if err := d.ensureSchemaMigrationsTable(ctx); err != nil {
		return nil, err
	}

	rows, err := d.Pool.Query(ctx, `SELECT version, applied_at FROM schema_migrations ORDER BY version ASC`)
	if err != nil {
		return nil, fmt.Errorf("querying schema_migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]time.Time)
	for rows.Next() {
		var version string
		var appliedAt time.Time
		if err := rows.Scan(&version, &appliedAt); err != nil {
			return nil, err
		}
		applied[version] = appliedAt
	}
	return applied, nil
}

// MigrateUp executes all pending *.up.sql migrations in version order inside transactions.
func (d *DB) MigrateUp(ctx context.Context) (int, error) {
	migrations, err := LoadEmbeddedMigrations()
	if err != nil {
		return 0, err
	}
	applied, err := d.getAppliedMigrations(ctx)
	if err != nil {
		return 0, err
	}

	appliedCount := 0
	for _, m := range migrations {
		if _, already := applied[m.Version]; already {
			continue
		}

		tx, err := d.Pool.Begin(ctx)
		if err != nil {
			return appliedCount, fmt.Errorf("starting tx for migration %s: %w", m.Version, err)
		}

		if _, err := tx.Exec(ctx, m.UpSQL); err != nil {
			_ = tx.Rollback(ctx)
			return appliedCount, fmt.Errorf("executing %s.up.sql: %w", m.Version, err)
		}

		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version, applied_at) VALUES ($1, NOW())`, m.Version); err != nil {
			_ = tx.Rollback(ctx)
			return appliedCount, fmt.Errorf("recording migration %s: %w", m.Version, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return appliedCount, fmt.Errorf("committing migration %s: %w", m.Version, err)
		}
		appliedCount++
	}

	return appliedCount, nil
}

// MigrateDown rolls back the most recently applied migration using its *.down.sql file.
func (d *DB) MigrateDown(ctx context.Context) (string, error) {
	migrations, err := LoadEmbeddedMigrations()
	if err != nil {
		return "", err
	}
	applied, err := d.getAppliedMigrations(ctx)
	if err != nil {
		return "", err
	}

	var latest *MigrationFile
	for i := len(migrations) - 1; i >= 0; i-- {
		if _, ok := applied[migrations[i].Version]; ok {
			m := migrations[i]
			latest = &m
			break
		}
	}

	if latest == nil {
		return "", nil
	}

	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("starting rollback tx for %s: %w", latest.Version, err)
	}

	if _, err := tx.Exec(ctx, latest.DownSQL); err != nil {
		_ = tx.Rollback(ctx)
		return "", fmt.Errorf("executing %s.down.sql: %w", latest.Version, err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM schema_migrations WHERE version = $1`, latest.Version); err != nil {
		_ = tx.Rollback(ctx)
		return "", fmt.Errorf("deleting migration record %s: %w", latest.Version, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("committing rollback %s: %w", latest.Version, err)
	}

	return latest.Version, nil
}

// MigrationStatus returns the status of every embedded migration against schema_migrations.
func (d *DB) MigrationStatus(ctx context.Context) ([]MigrationStatusRow, error) {
	migrations, err := LoadEmbeddedMigrations()
	if err != nil {
		return nil, err
	}
	applied, err := d.getAppliedMigrations(ctx)
	if err != nil {
		return nil, err
	}

	rows := make([]MigrationStatusRow, 0, len(migrations))
	for _, m := range migrations {
		row := MigrationStatusRow{Version: m.Version}
		if ts, ok := applied[m.Version]; ok {
			tCopy := ts
			row.Applied = true
			row.AppliedAt = &tCopy
		}
		rows = append(rows, row)
	}
	return rows, nil
}
