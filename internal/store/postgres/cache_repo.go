package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CacheRepo struct {
	pool *pgxpool.Pool
}

func NewCacheRepo(pool *pgxpool.Pool) *CacheRepo {
	return &CacheRepo{pool: pool}
}

func (r *CacheRepo) Get(ctx context.Context, cacheKey string) ([]byte, string, time.Time, bool, error) {
	query := `
		SELECT data, etag, expires_at
		FROM esi_cache
		WHERE cache_key = $1`

	var data []byte
	var etag string
	var expiresAt time.Time

	err := r.pool.QueryRow(ctx, query, cacheKey).Scan(&data, &etag, &expiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, "", time.Time{}, false, nil
		}
		return nil, "", time.Time{}, false, fmt.Errorf("failed to get esi cache: %w", err)
	}

	return data, etag, expiresAt, true, nil
}

func (r *CacheRepo) Set(ctx context.Context, cacheKey string, data []byte, etag string, expiresAt time.Time) error {
	query := `
		INSERT INTO esi_cache (cache_key, data, etag, expires_at, created_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (cache_key) DO UPDATE SET
			data = EXCLUDED.data,
			etag = EXCLUDED.etag,
			expires_at = EXCLUDED.expires_at,
			created_at = NOW()`

	_, err := r.pool.Exec(ctx, query, cacheKey, data, etag, expiresAt)
	return err
}

func (r *CacheRepo) CleanExpired(ctx context.Context) error {
	query := `DELETE FROM esi_cache WHERE expires_at < NOW()`
	_, err := r.pool.Exec(ctx, query)
	return err
}
