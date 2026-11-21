package postgres

import (
	"context"
	"fmt"
	"time"

	"aura-forge/internal/domain"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PIRepo implements domain.PIRepository backed by PostgreSQL.
type PIRepo struct {
	pool *pgxpool.Pool
}

// NewPIRepo creates a new PostgreSQL-backed PIRepository.
func NewPIRepo(pool *pgxpool.Pool) *PIRepo {
	return &PIRepo{pool: pool}
}

func (r *PIRepo) ListColonies(ctx context.Context) ([]domain.PIColony, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, planet_name, solar_system, planet_type, product_type_id, product_name, tier, extractor_expires_at, est_daily_isk, note, created_at, updated_at
		FROM pi_colonies
		ORDER BY extractor_expires_at ASC, id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("listing pi_colonies: %w", err)
	}
	defer rows.Close()

	out := make([]domain.PIColony, 0)
	for rows.Next() {
		var c domain.PIColony
		var tier string
		if err := rows.Scan(
			&c.ID, &c.PlanetName, &c.SolarSystem, &c.PlanetType,
			&c.ProductTypeID, &c.ProductName, &tier, &c.ExtractorExpiresAt,
			&c.EstDailyISK, &c.Note, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning pi_colony: %w", err)
		}
		c.Tier = domain.PITier(tier)
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *PIRepo) SaveColony(ctx context.Context, c *domain.PIColony) error {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO pi_colonies (planet_name, solar_system, planet_type, product_type_id, product_name, tier, extractor_expires_at, est_daily_isk, note, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`, c.PlanetName, c.SolarSystem, c.PlanetType, c.ProductTypeID, c.ProductName, string(c.Tier), c.ExtractorExpiresAt, c.EstDailyISK, c.Note).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return fmt.Errorf("saving pi_colony: %w", err)
	}
	return nil
}

func (r *PIRepo) ResetExtractor(ctx context.Context, id int64, newExpiresAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE pi_colonies
		SET extractor_expires_at = $2, updated_at = NOW()
		WHERE id = $1
	`, id, newExpiresAt)
	if err != nil {
		return fmt.Errorf("resetting pi_colony extractor %d: %w", id, err)
	}
	return nil
}

func (r *PIRepo) DeleteColony(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM pi_colonies WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting pi_colony %d: %w", id, err)
	}
	return nil
}
