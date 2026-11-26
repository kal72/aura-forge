package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB wraps pgxpool.Pool and provides database connectivity and versioned migration execution.
type DB struct {
	Pool *pgxpool.Pool
}

// NewPoolOnly connects to PostgreSQL and verifies the connection without running auto-migrations.
func NewPoolOnly(ctx context.Context, databaseURL string) (*DB, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database config: %w", err)
	}

	config.MaxConns = 25
	config.MinConns = 2
	config.MaxConnLifetime = 1 * time.Hour
	config.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	const maxPingAttempts = 3
	var lastPingErr error
	for attempt := 1; attempt <= maxPingAttempts; attempt++ {
		pingCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		lastPingErr = pool.Ping(pingCtx)
		cancel()
		if lastPingErr == nil {
			break
		}
		if attempt < maxPingAttempts {
			select {
			case <-ctx.Done():
				pool.Close()
				return nil, fmt.Errorf("database ping cancelled: %w", ctx.Err())
			case <-time.After(time.Duration(attempt*2) * time.Second):
			}
		}
	}
	if lastPingErr != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database after %d attempts: %w", maxPingAttempts, lastPingErr)
	}

	return &DB{Pool: pool}, nil
}

// NewDB initializes a connection pool to PostgreSQL and runs all pending versioned migrations.
func NewDB(ctx context.Context, databaseURL string) (*DB, error) {
	db, err := NewPoolOnly(ctx, databaseURL)
	if err != nil {
		return nil, err
	}

	if err := db.Migrate(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}

// Migrate executes all pending versioned *.up.sql migrations recorded in schema_migrations.
func (d *DB) Migrate(ctx context.Context) error {
	_, err := d.MigrateUp(ctx)
	return err
}

// Close closes the connection pool.
func (d *DB) Close() {
	if d.Pool != nil {
		d.Pool.Close()
	}
}
