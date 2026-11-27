package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"aura-forge/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TacticalRepo implements domain.TacticalRepository backed by PostgreSQL.
type TacticalRepo struct {
	pool *pgxpool.Pool
}

// NewTacticalRepo constructs a PostgreSQL-backed TacticalRepository.
func NewTacticalRepo(pool *pgxpool.Pool) *TacticalRepo {
	return &TacticalRepo{pool: pool}
}

func (r *TacticalRepo) ListFittings(ctx context.Context, userID int64, limit int) ([]domain.TacticalFittingRecord, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var (
		rows pgx.Rows
		err  error
	)
	if userID > 0 {
		rows, err = r.pool.Query(ctx, `
			SELECT id, COALESCE(user_id, 0), name, ship_type_id, ship_name, hull_class, tag, skill_profile,
			       eft_text, fit_json, estimated_cost_isk, ehp, dps, created_at, updated_at
			FROM tactical_fittings
			WHERE user_id = $1 OR user_id IS NULL
			ORDER BY updated_at DESC
			LIMIT $2
		`, userID, limit)
	} else {
		rows, err = r.pool.Query(ctx, `
			SELECT id, COALESCE(user_id, 0), name, ship_type_id, ship_name, hull_class, tag, skill_profile,
			       eft_text, fit_json, estimated_cost_isk, ehp, dps, created_at, updated_at
			FROM tactical_fittings
			ORDER BY updated_at DESC
			LIMIT $1
		`, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("listing tactical_fittings: %w", err)
	}
	defer rows.Close()

	out := make([]domain.TacticalFittingRecord, 0)
	for rows.Next() {
		var rec domain.TacticalFittingRecord
		var fitBytes []byte
		if err := rows.Scan(
			&rec.ID, &rec.UserID, &rec.Name, &rec.ShipTypeID, &rec.ShipName, &rec.HullClass,
			&rec.Tag, &rec.SkillProfile, &rec.EFTText, &fitBytes,
			&rec.EstimatedCostISK, &rec.EHP, &rec.DPS, &rec.CreatedAt, &rec.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning tactical_fittings: %w", err)
		}
		if len(fitBytes) > 0 {
			_ = json.Unmarshal(fitBytes, &rec.Fit)
			rec.Fit.ID = rec.ID
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (r *TacticalRepo) GetFitting(ctx context.Context, id int64, userID int64) (*domain.TacticalFittingRecord, error) {
	var rec domain.TacticalFittingRecord
	var fitBytes []byte
	var err error
	if userID > 0 {
		err = r.pool.QueryRow(ctx, `
			SELECT id, COALESCE(user_id, 0), name, ship_type_id, ship_name, hull_class, tag, skill_profile,
			       eft_text, fit_json, estimated_cost_isk, ehp, dps, created_at, updated_at
			FROM tactical_fittings
			WHERE id = $1 AND (user_id = $2 OR user_id IS NULL)
		`, id, userID).Scan(
			&rec.ID, &rec.UserID, &rec.Name, &rec.ShipTypeID, &rec.ShipName, &rec.HullClass,
			&rec.Tag, &rec.SkillProfile, &rec.EFTText, &fitBytes,
			&rec.EstimatedCostISK, &rec.EHP, &rec.DPS, &rec.CreatedAt, &rec.UpdatedAt,
		)
	} else {
		err = r.pool.QueryRow(ctx, `
			SELECT id, COALESCE(user_id, 0), name, ship_type_id, ship_name, hull_class, tag, skill_profile,
			       eft_text, fit_json, estimated_cost_isk, ehp, dps, created_at, updated_at
			FROM tactical_fittings
			WHERE id = $1
		`, id).Scan(
			&rec.ID, &rec.UserID, &rec.Name, &rec.ShipTypeID, &rec.ShipName, &rec.HullClass,
			&rec.Tag, &rec.SkillProfile, &rec.EFTText, &fitBytes,
			&rec.EstimatedCostISK, &rec.EHP, &rec.DPS, &rec.CreatedAt, &rec.UpdatedAt,
		)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting tactical_fitting %d: %w", id, err)
	}
	if len(fitBytes) > 0 {
		_ = json.Unmarshal(fitBytes, &rec.Fit)
		rec.Fit.ID = rec.ID
	}
	return &rec, nil
}

func (r *TacticalRepo) SaveFitting(ctx context.Context, rec *domain.TacticalFittingRecord) (*domain.TacticalFittingRecord, error) {
	if rec == nil {
		return nil, fmt.Errorf("fitting record is nil")
	}
	fitJSON, err := json.Marshal(rec.Fit)
	if err != nil {
		return nil, fmt.Errorf("marshaling fit_json: %w", err)
	}
	var uid any
	if rec.UserID > 0 {
		uid = rec.UserID
	}

	if rec.ID > 0 {
		err = r.pool.QueryRow(ctx, `
			UPDATE tactical_fittings
			SET name = $2, ship_type_id = $3, ship_name = $4, hull_class = $5,
			    tag = $6, skill_profile = $7, eft_text = $8, fit_json = $9,
			    estimated_cost_isk = $10, ehp = $11, dps = $12, updated_at = NOW()
			WHERE id = $1 AND ($13::bigint IS NULL OR user_id = $13 OR user_id IS NULL)
			RETURNING id, created_at, updated_at
		`, rec.ID, rec.Name, rec.ShipTypeID, rec.ShipName, rec.HullClass,
			rec.Tag, rec.SkillProfile, rec.EFTText, fitJSON,
			rec.EstimatedCostISK, rec.EHP, rec.DPS, uid,
		).Scan(&rec.ID, &rec.CreatedAt, &rec.UpdatedAt)
		if err == nil {
			rec.Fit.ID = rec.ID
			return rec, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("updating tactical_fitting %d: %w", rec.ID, err)
		}
	}

	err = r.pool.QueryRow(ctx, `
		INSERT INTO tactical_fittings
			(user_id, name, ship_type_id, ship_name, hull_class, tag, skill_profile,
			 eft_text, fit_json, estimated_cost_isk, ehp, dps)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, created_at, updated_at
	`, uid, rec.Name, rec.ShipTypeID, rec.ShipName, rec.HullClass,
		rec.Tag, rec.SkillProfile, rec.EFTText, fitJSON,
		rec.EstimatedCostISK, rec.EHP, rec.DPS,
	).Scan(&rec.ID, &rec.CreatedAt, &rec.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("inserting tactical_fitting: %w", err)
	}
	rec.Fit.ID = rec.ID
	return rec, nil
}

func (r *TacticalRepo) DeleteFitting(ctx context.Context, id int64, userID int64) error {
	var err error
	if userID > 0 {
		_, err = r.pool.Exec(ctx, `DELETE FROM tactical_fittings WHERE id = $1 AND (user_id = $2 OR user_id IS NULL)`, id, userID)
	} else {
		_, err = r.pool.Exec(ctx, `DELETE FROM tactical_fittings WHERE id = $1`, id)
	}
	if err != nil {
		return fmt.Errorf("deleting tactical_fitting %d: %w", id, err)
	}
	return nil
}

func (r *TacticalRepo) GetAICache(ctx context.Context, feature, contextHash string) ([]byte, bool, error) {
	var payload []byte
	err := r.pool.QueryRow(ctx, `
		SELECT payload_json FROM ai_tactical_cache
		WHERE feature = $1 AND context_hash = $2 AND expires_at > NOW()
	`, feature, contextHash).Scan(&payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("reading ai_tactical_cache: %w", err)
	}
	return payload, true, nil
}

func (r *TacticalRepo) SaveAICache(ctx context.Context, feature, contextHash, model string, payload []byte, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = 6 * time.Hour
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO ai_tactical_cache (feature, context_hash, model, payload_json, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (feature, context_hash)
		DO UPDATE SET model = EXCLUDED.model, payload_json = EXCLUDED.payload_json, expires_at = EXCLUDED.expires_at, created_at = NOW()
	`, feature, contextHash, model, payload, time.Now().Add(ttl))
	if err != nil {
		return fmt.Errorf("saving ai_tactical_cache: %w", err)
	}
	return nil
}
