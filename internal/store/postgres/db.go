package postgres

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/001_init.sql
var initSQL string

// DB wraps pgxpool.Pool and provides database connectivity and migration execution.
type DB struct {
	Pool *pgxpool.Pool
}

// NewDB initializes a connection pool to PostgreSQL and runs pending migrations.
func NewDB(ctx context.Context, databaseURL string) (*DB, error) {
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

	// Verify connection
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db := &DB{Pool: pool}
	if err := db.Migrate(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}

// Migrate executes initial schema SQL.
func (d *DB) Migrate(ctx context.Context) error {
	_, err := d.Pool.Exec(ctx, initSQL)
	if err != nil {
		return fmt.Errorf("executing migration 001_init.sql failed: %w", err)
	}
	return nil
}

// Close closes the pool.
func (d *DB) Close() {
	if d.Pool != nil {
		d.Pool.Close()
	}
}
