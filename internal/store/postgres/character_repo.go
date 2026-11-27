package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"aura-forge/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CharacterRepo struct {
	pool *pgxpool.Pool
}

func NewCharacterRepo(pool *pgxpool.Pool) *CharacterRepo {
	return &CharacterRepo{pool: pool}
}

// Profile
func (r *CharacterRepo) GetProfile(ctx context.Context, characterID int64) (*domain.CharacterProfile, error) {
	query := `
		SELECT character_id, character_name, wallet_balance,
		       total_manufacturing_slots, used_manufacturing_slots,
		       total_science_slots, used_science_slots, updated_at
		FROM character_profiles
		WHERE character_id = $1`

	var p domain.CharacterProfile
	err := r.pool.QueryRow(ctx, query, characterID).Scan(
		&p.CharacterID, &p.CharacterName, &p.WalletBalance,
		&p.TotalManufacturingSlots, &p.UsedManufacturingSlots,
		&p.TotalScienceSlots, &p.UsedScienceSlots, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get character profile: %w", err)
	}

	return &p, nil
}

func (r *CharacterRepo) UpsertProfile(ctx context.Context, p *domain.CharacterProfile) error {
	query := `
		INSERT INTO character_profiles (
			character_id, character_name, wallet_balance,
			total_manufacturing_slots, used_manufacturing_slots,
			total_science_slots, used_science_slots, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (character_id) DO UPDATE SET
			character_name = EXCLUDED.character_name,
			wallet_balance = EXCLUDED.wallet_balance,
			total_manufacturing_slots = EXCLUDED.total_manufacturing_slots,
			used_manufacturing_slots = EXCLUDED.used_manufacturing_slots,
			total_science_slots = EXCLUDED.total_science_slots,
			used_science_slots = EXCLUDED.used_science_slots,
			updated_at = EXCLUDED.updated_at`

	p.UpdatedAt = time.Now()
	_, err := r.pool.Exec(ctx, query,
		p.CharacterID, p.CharacterName, p.WalletBalance,
		p.TotalManufacturingSlots, p.UsedManufacturingSlots,
		p.TotalScienceSlots, p.UsedScienceSlots, p.UpdatedAt,
	)
	return err
}

// Token
func (r *CharacterRepo) GetToken(ctx context.Context, characterID int64) (*domain.EVEToken, error) {
	query := `
		SELECT character_id, character_name, access_token, refresh_token, expires_at, scope, updated_at
		FROM eve_tokens
		WHERE character_id = $1`

	var t domain.EVEToken
	err := r.pool.QueryRow(ctx, query, characterID).Scan(
		&t.CharacterID, &t.CharacterName, &t.AccessToken, &t.RefreshToken,
		&t.ExpiresAt, &t.Scope, &t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get token: %w", err)
	}
	return &t, nil
}

func (r *CharacterRepo) GetPrimaryToken(ctx context.Context) (*domain.EVEToken, error) {
	query := `
		SELECT character_id, character_name, access_token, refresh_token, expires_at, scope, updated_at
		FROM eve_tokens
		ORDER BY updated_at DESC
		LIMIT 1`

	var t domain.EVEToken
	err := r.pool.QueryRow(ctx, query).Scan(
		&t.CharacterID, &t.CharacterName, &t.AccessToken, &t.RefreshToken,
		&t.ExpiresAt, &t.Scope, &t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get primary token: %w", err)
	}
	return &t, nil
}

func (r *CharacterRepo) SaveToken(ctx context.Context, token *domain.EVEToken) error {
	query := `
		INSERT INTO eve_tokens (
			character_id, character_name, access_token, refresh_token, expires_at, scope, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (character_id) DO UPDATE SET
			character_name = EXCLUDED.character_name,
			access_token = EXCLUDED.access_token,
			refresh_token = EXCLUDED.refresh_token,
			expires_at = EXCLUDED.expires_at,
			scope = EXCLUDED.scope,
			updated_at = EXCLUDED.updated_at`

	token.UpdatedAt = time.Now()
	_, err := r.pool.Exec(ctx, query,
		token.CharacterID, token.CharacterName, token.AccessToken,
		token.RefreshToken, token.ExpiresAt, token.Scope, token.UpdatedAt,
	)
	return err
}

func (r *CharacterRepo) DeleteToken(ctx context.Context, characterID int64) error {
	query := `DELETE FROM eve_tokens WHERE character_id = $1`
	_, err := r.pool.Exec(ctx, query, characterID)
	return err
}

// Orders
func (r *CharacterRepo) ListOrders(ctx context.Context, characterID int64) ([]domain.CharacterOrder, error) {
	query := `
		SELECT order_id, character_id, type_id, type_name, price, volume_remain,
		       volume_total, is_buy_order, region_id, location_id, issued_at,
		       is_undercut, competitor_lowest_price, safe_relist_price, updated_at
		FROM character_orders
		WHERE character_id = $1
		ORDER BY issued_at DESC`

	rows, err := r.pool.Query(ctx, query, characterID)
	if err != nil {
		return nil, fmt.Errorf("failed to query orders: %w", err)
	}
	defer rows.Close()

	var orders []domain.CharacterOrder
	for rows.Next() {
		var o domain.CharacterOrder
		err := rows.Scan(
			&o.OrderID, &o.CharacterID, &o.TypeID, &o.TypeName, &o.Price, &o.VolumeRemain,
			&o.VolumeTotal, &o.IsBuyOrder, &o.RegionID, &o.LocationID, &o.IssuedAt,
			&o.IsUndercut, &o.CompetitorLowestPrice, &o.SafeRelistPrice, &o.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan order: %w", err)
		}
		orders = append(orders, o)
	}

	return orders, nil
}

func (r *CharacterRepo) UpsertOrders(ctx context.Context, characterID int64, orders []domain.CharacterOrder) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Clean out existing orders for this character and insert current active
	_, err = tx.Exec(ctx, `DELETE FROM character_orders WHERE character_id = $1`, characterID)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO character_orders (
			order_id, character_id, type_id, type_name, price, volume_remain,
			volume_total, is_buy_order, region_id, location_id, issued_at,
			is_undercut, competitor_lowest_price, safe_relist_price, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`

	now := time.Now()
	for _, o := range orders {
		_, err := tx.Exec(ctx, query,
			o.OrderID, characterID, o.TypeID, o.TypeName, o.Price, o.VolumeRemain,
			o.VolumeTotal, o.IsBuyOrder, o.RegionID, o.LocationID, o.IssuedAt,
			o.IsUndercut, o.CompetitorLowestPrice, o.SafeRelistPrice, now,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// Jobs
func (r *CharacterRepo) ListJobs(ctx context.Context, characterID int64) ([]domain.CharacterJob, error) {
	query := `
		SELECT job_id, character_id, type_id, type_name, activity_id, activity_name,
		       runs, status, start_date, end_date, completed_date, updated_at
		FROM character_jobs
		WHERE character_id = $1
		ORDER BY end_date ASC`

	rows, err := r.pool.Query(ctx, query, characterID)
	if err != nil {
		return nil, fmt.Errorf("failed to query jobs: %w", err)
	}
	defer rows.Close()

	var jobs []domain.CharacterJob
	for rows.Next() {
		var j domain.CharacterJob
		err := rows.Scan(
			&j.JobID, &j.CharacterID, &j.TypeID, &j.TypeName, &j.ActivityID, &j.ActivityName,
			&j.Runs, &j.Status, &j.StartDate, &j.EndDate, &j.CompletedDate, &j.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan job: %w", err)
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
}

func (r *CharacterRepo) UpsertJobs(ctx context.Context, characterID int64, jobs []domain.CharacterJob) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `DELETE FROM character_jobs WHERE character_id = $1`, characterID)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO character_jobs (
			job_id, character_id, type_id, type_name, activity_id, activity_name,
			runs, status, start_date, end_date, completed_date, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

	now := time.Now()
	for _, j := range jobs {
		_, err := tx.Exec(ctx, query,
			j.JobID, characterID, j.TypeID, j.TypeName, j.ActivityID, j.ActivityName,
			j.Runs, j.Status, j.StartDate, j.EndDate, j.CompletedDate, now,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// Blueprints (Personal Character)
func (r *CharacterRepo) ListBlueprints(ctx context.Context, characterID int64) ([]domain.CharacterBlueprint, error) {
	return r.listBlueprintsByScope(ctx, characterID, false)
}

// ListCorpBlueprints returns all synced corporation blueprints accessible to the character.
func (r *CharacterRepo) ListCorpBlueprints(ctx context.Context, characterID int64) ([]domain.CharacterBlueprint, error) {
	return r.listBlueprintsByScope(ctx, characterID, true)
}

func (r *CharacterRepo) listBlueprintsByScope(ctx context.Context, characterID int64, isCorp bool) ([]domain.CharacterBlueprint, error) {
	query := `
		SELECT item_id, character_id, type_id, type_name, location_id, location_flag, quantity,
		       material_efficiency, time_efficiency, runs, is_copy, is_corp, updated_at
		FROM character_blueprints
		WHERE character_id = $1 AND is_corp = $2
		ORDER BY type_name ASC`

	rows, err := r.pool.Query(ctx, query, characterID, isCorp)
	if err != nil {
		return nil, fmt.Errorf("failed to query blueprints: %w", err)
	}
	defer rows.Close()

	var bps []domain.CharacterBlueprint
	for rows.Next() {
		var bp domain.CharacterBlueprint
		err := rows.Scan(
			&bp.ItemID, &bp.CharacterID, &bp.TypeID, &bp.TypeName, &bp.LocationID, &bp.LocationFlag,
			&bp.Quantity, &bp.MaterialEfficiency, &bp.TimeEfficiency, &bp.Runs,
			&bp.IsCopy, &bp.IsCorp, &bp.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan blueprint: %w", err)
		}
		bps = append(bps, bp)
	}
	return bps, nil
}

func (r *CharacterRepo) UpsertBlueprints(ctx context.Context, characterID int64, bps []domain.CharacterBlueprint) error {
	return r.upsertBlueprintsByScope(ctx, characterID, false, bps)
}

func (r *CharacterRepo) UpsertCorpBlueprints(ctx context.Context, characterID int64, bps []domain.CharacterBlueprint) error {
	return r.upsertBlueprintsByScope(ctx, characterID, true, bps)
}

func (r *CharacterRepo) upsertBlueprintsByScope(ctx context.Context, characterID int64, isCorp bool, bps []domain.CharacterBlueprint) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `DELETE FROM character_blueprints WHERE character_id = $1 AND is_corp = $2`, characterID, isCorp)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO character_blueprints (
			item_id, character_id, type_id, type_name, location_id, location_flag, quantity,
			material_efficiency, time_efficiency, runs, is_copy, is_corp, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (item_id) DO UPDATE SET
			character_id = EXCLUDED.character_id,
			type_id = EXCLUDED.type_id,
			type_name = EXCLUDED.type_name,
			location_id = EXCLUDED.location_id,
			location_flag = EXCLUDED.location_flag,
			quantity = EXCLUDED.quantity,
			material_efficiency = EXCLUDED.material_efficiency,
			time_efficiency = EXCLUDED.time_efficiency,
			runs = EXCLUDED.runs,
			is_copy = EXCLUDED.is_copy,
			is_corp = EXCLUDED.is_corp,
			updated_at = EXCLUDED.updated_at`

	now := time.Now()
	for _, bp := range bps {
		_, err := tx.Exec(ctx, query,
			bp.ItemID, characterID, bp.TypeID, bp.TypeName, bp.LocationID, bp.LocationFlag, bp.Quantity,
			bp.MaterialEfficiency, bp.TimeEfficiency, bp.Runs, bp.IsCopy, isCorp, now,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// Assets
func (r *CharacterRepo) ListAssets(ctx context.Context, characterID int64) ([]domain.HangarAsset, error) {
	query := `
		SELECT item_id, character_id, type_id, type_name, location_id, quantity, updated_at
		FROM character_assets
		WHERE character_id = $1
		ORDER BY type_name ASC`

	rows, err := r.pool.Query(ctx, query, characterID)
	if err != nil {
		return nil, fmt.Errorf("failed to query assets: %w", err)
	}
	defer rows.Close()

	var assets []domain.HangarAsset
	for rows.Next() {
		var a domain.HangarAsset
		err := rows.Scan(
			&a.ItemID, &a.CharacterID, &a.TypeID, &a.TypeName, &a.LocationID, &a.Quantity, &a.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan asset: %w", err)
		}
		assets = append(assets, a)
	}
	return assets, nil
}

func (r *CharacterRepo) UpsertAssets(ctx context.Context, characterID int64, assets []domain.HangarAsset) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `DELETE FROM character_assets WHERE character_id = $1`, characterID)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO character_assets (
			item_id, character_id, type_id, type_name, location_id, quantity, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)`

	now := time.Now()
	for _, a := range assets {
		_, err := tx.Exec(ctx, query,
			a.ItemID, characterID, a.TypeID, a.TypeName, a.LocationID, a.Quantity, now,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}
