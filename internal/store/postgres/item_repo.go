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

type ItemRepo struct {
	pool *pgxpool.Pool
}

func NewItemRepo(pool *pgxpool.Pool) *ItemRepo {
	return &ItemRepo{pool: pool}
}

func (r *ItemRepo) ListItems(ctx context.Context) ([]domain.MarketItem, error) {
	query := `
		SELECT id, type_id, name, category, estimated_jita_sell, estimated_jita_buy,
		       daily_volume, volume_30d, material_cost, batch_output_qty,
		       production_time_seconds, is_manufacturable, requires_rare_bpc,
		       extra_setup_cost, is_frequently_destroyed, trap_analysis,
		       created_at, updated_at
		FROM market_items
		ORDER BY name ASC`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query market items: %w", err)
	}
	defer rows.Close()

	var items []domain.MarketItem
	for rows.Next() {
		var item domain.MarketItem
		var trapJSON []byte
		err := rows.Scan(
			&item.ID, &item.TypeID, &item.Name, &item.Category, &item.EstimatedJitaSell, &item.EstimatedJitaBuy,
			&item.DailyVolume, &item.Volume30d, &item.MaterialCost, &item.BatchOutputQty,
			&item.ProductionTimeSeconds, &item.IsManufacturable, &item.RequiresRareBPC,
			&item.ExtraSetupCost, &item.IsFrequentlyDestroyed, &trapJSON,
			&item.CreatedAt, &item.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan item: %w", err)
		}
		if len(trapJSON) > 0 {
			_ = json.Unmarshal(trapJSON, &item.TrapAnalysis)
		}
		items = append(items, item)
	}

	return items, nil
}

func (r *ItemRepo) GetItemByID(ctx context.Context, id int64) (*domain.MarketItem, error) {
	query := `
		SELECT id, type_id, name, category, estimated_jita_sell, estimated_jita_buy,
		       daily_volume, volume_30d, material_cost, batch_output_qty,
		       production_time_seconds, is_manufacturable, requires_rare_bpc,
		       extra_setup_cost, is_frequently_destroyed, trap_analysis,
		       created_at, updated_at
		FROM market_items
		WHERE id = $1`

	var item domain.MarketItem
	var trapJSON []byte
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&item.ID, &item.TypeID, &item.Name, &item.Category, &item.EstimatedJitaSell, &item.EstimatedJitaBuy,
		&item.DailyVolume, &item.Volume30d, &item.MaterialCost, &item.BatchOutputQty,
		&item.ProductionTimeSeconds, &item.IsManufacturable, &item.RequiresRareBPC,
		&item.ExtraSetupCost, &item.IsFrequentlyDestroyed, &trapJSON,
		&item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get item by id: %w", err)
	}
	if len(trapJSON) > 0 {
		_ = json.Unmarshal(trapJSON, &item.TrapAnalysis)
	}

	return &item, nil
}

func (r *ItemRepo) GetItemByTypeID(ctx context.Context, typeID int32) (*domain.MarketItem, error) {
	query := `
		SELECT id, type_id, name, category, estimated_jita_sell, estimated_jita_buy,
		       daily_volume, volume_30d, material_cost, batch_output_qty,
		       production_time_seconds, is_manufacturable, requires_rare_bpc,
		       extra_setup_cost, is_frequently_destroyed, trap_analysis,
		       created_at, updated_at
		FROM market_items
		WHERE type_id = $1`

	var item domain.MarketItem
	var trapJSON []byte
	err := r.pool.QueryRow(ctx, query, typeID).Scan(
		&item.ID, &item.TypeID, &item.Name, &item.Category, &item.EstimatedJitaSell, &item.EstimatedJitaBuy,
		&item.DailyVolume, &item.Volume30d, &item.MaterialCost, &item.BatchOutputQty,
		&item.ProductionTimeSeconds, &item.IsManufacturable, &item.RequiresRareBPC,
		&item.ExtraSetupCost, &item.IsFrequentlyDestroyed, &trapJSON,
		&item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get item by type_id: %w", err)
	}
	if len(trapJSON) > 0 {
		_ = json.Unmarshal(trapJSON, &item.TrapAnalysis)
	}

	return &item, nil
}

func (r *ItemRepo) SaveItem(ctx context.Context, item *domain.MarketItem) error {
	trapJSON, err := json.Marshal(item.TrapAnalysis)
	if err != nil {
		trapJSON = []byte("{}")
	}

	now := time.Now()
	item.UpdatedAt = now

	if item.ID == 0 {
		query := `
			INSERT INTO market_items (
				type_id, name, category, estimated_jita_sell, estimated_jita_buy,
				daily_volume, volume_30d, material_cost, batch_output_qty,
				production_time_seconds, is_manufacturable, requires_rare_bpc,
				extra_setup_cost, is_frequently_destroyed, trap_analysis,
				created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
			ON CONFLICT (type_id) DO UPDATE SET
				name = EXCLUDED.name,
				category = EXCLUDED.category,
				estimated_jita_sell = EXCLUDED.estimated_jita_sell,
				estimated_jita_buy = EXCLUDED.estimated_jita_buy,
				daily_volume = EXCLUDED.daily_volume,
				volume_30d = EXCLUDED.volume_30d,
				material_cost = EXCLUDED.material_cost,
				batch_output_qty = EXCLUDED.batch_output_qty,
				production_time_seconds = EXCLUDED.production_time_seconds,
				is_manufacturable = EXCLUDED.is_manufacturable,
				requires_rare_bpc = EXCLUDED.requires_rare_bpc,
				extra_setup_cost = EXCLUDED.extra_setup_cost,
				is_frequently_destroyed = EXCLUDED.is_frequently_destroyed,
				trap_analysis = EXCLUDED.trap_analysis,
				updated_at = EXCLUDED.updated_at
			RETURNING id, created_at`

		return r.pool.QueryRow(ctx, query,
			item.TypeID, item.Name, item.Category, item.EstimatedJitaSell, item.EstimatedJitaBuy,
			item.DailyVolume, item.Volume30d, item.MaterialCost, item.BatchOutputQty,
			item.ProductionTimeSeconds, item.IsManufacturable, item.RequiresRareBPC,
			item.ExtraSetupCost, item.IsFrequentlyDestroyed, trapJSON,
			now, now,
		).Scan(&item.ID, &item.CreatedAt)
	}

	query := `
		UPDATE market_items SET
			type_id = $1, name = $2, category = $3, estimated_jita_sell = $4,
			estimated_jita_buy = $5, daily_volume = $6, volume_30d = $7,
			material_cost = $8, batch_output_qty = $9, production_time_seconds = $10,
			is_manufacturable = $11, requires_rare_bpc = $12, extra_setup_cost = $13,
			is_frequently_destroyed = $14, trap_analysis = $15, updated_at = $16
		WHERE id = $17`

	_, err = r.pool.Exec(ctx, query,
		item.TypeID, item.Name, item.Category, item.EstimatedJitaSell,
		item.EstimatedJitaBuy, item.DailyVolume, item.Volume30d,
		item.MaterialCost, item.BatchOutputQty, item.ProductionTimeSeconds,
		item.IsManufacturable, item.RequiresRareBPC, item.ExtraSetupCost,
		item.IsFrequentlyDestroyed, trapJSON, now, item.ID,
	)
	return err
}

func (r *ItemRepo) DeleteItem(ctx context.Context, id int64) error {
	query := `DELETE FROM market_items WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, id)
	return err
}

func (r *ItemRepo) BatchUpsertItems(ctx context.Context, items []domain.MarketItem) error {
	if len(items) == 0 {
		return nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("starting batch upsert tx: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO market_items (
			type_id, name, category, estimated_jita_sell, estimated_jita_buy,
			daily_volume, volume_30d, material_cost, batch_output_qty,
			production_time_seconds, is_manufacturable, requires_rare_bpc,
			extra_setup_cost, is_frequently_destroyed, trap_analysis,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		ON CONFLICT (type_id) DO UPDATE SET
			name = EXCLUDED.name,
			category = EXCLUDED.category,
			estimated_jita_sell = EXCLUDED.estimated_jita_sell,
			estimated_jita_buy = EXCLUDED.estimated_jita_buy,
			daily_volume = EXCLUDED.daily_volume,
			volume_30d = EXCLUDED.volume_30d,
			material_cost = EXCLUDED.material_cost,
			batch_output_qty = EXCLUDED.batch_output_qty,
			production_time_seconds = EXCLUDED.production_time_seconds,
			is_manufacturable = EXCLUDED.is_manufacturable,
			requires_rare_bpc = EXCLUDED.requires_rare_bpc,
			extra_setup_cost = EXCLUDED.extra_setup_cost,
			is_frequently_destroyed = EXCLUDED.is_frequently_destroyed,
			trap_analysis = EXCLUDED.trap_analysis,
			updated_at = EXCLUDED.updated_at`

	now := time.Now()
	for _, item := range items {
		trapJSON, err := json.Marshal(item.TrapAnalysis)
		if err != nil {
			trapJSON = []byte("{}")
		}

		_, err = tx.Exec(ctx, query,
			item.TypeID, item.Name, item.Category, item.EstimatedJitaSell, item.EstimatedJitaBuy,
			item.DailyVolume, item.Volume30d, item.MaterialCost, item.BatchOutputQty,
			item.ProductionTimeSeconds, item.IsManufacturable, item.RequiresRareBPC,
			item.ExtraSetupCost, item.IsFrequentlyDestroyed, trapJSON,
			now, now,
		)
		if err != nil {
			return fmt.Errorf("batch upserting item %d: %w", item.TypeID, err)
		}
	}

	return tx.Commit(ctx)
}
