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

type BatchRepo struct {
	pool *pgxpool.Pool
}

func NewBatchRepo(pool *pgxpool.Pool) *BatchRepo {
	return &BatchRepo{pool: pool}
}

func (r *BatchRepo) ListBatches(ctx context.Context, status *domain.BatchStatus) ([]domain.BatchRecord, error) {
	var query string
	var args []any

	if status != nil && *status != "" {
		query = `
			SELECT id, plan_id, entry_type, type_id, item_name, runs, units_output,
			       total_material_cost, actual_production_cost, actual_sell_price,
			       total_revenue, net_profit, roi, status, market_order_id,
			       current_market_price, is_undercut, safe_relist_price,
			       remaining_safe_margin, started_at, finished_at, listed_at, sold_at,
			       created_at, updated_at
			FROM production_batches
			WHERE status = $1
			ORDER BY created_at DESC`
		args = append(args, *status)
	} else {
		query = `
			SELECT id, plan_id, entry_type, type_id, item_name, runs, units_output,
			       total_material_cost, actual_production_cost, actual_sell_price,
			       total_revenue, net_profit, roi, status, market_order_id,
			       current_market_price, is_undercut, safe_relist_price,
			       remaining_safe_margin, started_at, finished_at, listed_at, sold_at,
			       created_at, updated_at
			FROM production_batches
			ORDER BY created_at DESC`
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query batches: %w", err)
	}
	defer rows.Close()

	var batches []domain.BatchRecord
	for rows.Next() {
		var b domain.BatchRecord
		err := rows.Scan(
			&b.ID, &b.PlanID, &b.EntryType, &b.TypeID, &b.ItemName, &b.Runs, &b.UnitsOutput,
			&b.TotalMaterialCost, &b.ActualProductionCost, &b.ActualSellPrice,
			&b.TotalRevenue, &b.NetProfit, &b.ROI, &b.Status, &b.MarketOrderID,
			&b.CurrentMarketPrice, &b.IsUndercut, &b.SafeRelistPrice,
			&b.RemainingSafeMargin, &b.StartedAt, &b.FinishedAt, &b.ListedAt, &b.SoldAt,
			&b.CreatedAt, &b.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan batch: %w", err)
		}
		batches = append(batches, b)
	}

	return batches, nil
}

func (r *BatchRepo) GetBatchByID(ctx context.Context, id int64) (*domain.BatchRecord, error) {
	query := `
		SELECT id, plan_id, entry_type, type_id, item_name, runs, units_output,
		       total_material_cost, actual_production_cost, actual_sell_price,
		       total_revenue, net_profit, roi, status, market_order_id,
		       current_market_price, is_undercut, safe_relist_price,
		       remaining_safe_margin, started_at, finished_at, listed_at, sold_at,
		       created_at, updated_at
		FROM production_batches
		WHERE id = $1`

	var b domain.BatchRecord
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&b.ID, &b.PlanID, &b.EntryType, &b.TypeID, &b.ItemName, &b.Runs, &b.UnitsOutput,
		&b.TotalMaterialCost, &b.ActualProductionCost, &b.ActualSellPrice,
		&b.TotalRevenue, &b.NetProfit, &b.ROI, &b.Status, &b.MarketOrderID,
		&b.CurrentMarketPrice, &b.IsUndercut, &b.SafeRelistPrice,
		&b.RemainingSafeMargin, &b.StartedAt, &b.FinishedAt, &b.ListedAt, &b.SoldAt,
		&b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get batch: %w", err)
	}

	return &b, nil
}

func (r *BatchRepo) SaveBatch(ctx context.Context, batch *domain.BatchRecord) error {
	now := time.Now()
	batch.UpdatedAt = now
	batch.EntryType = batch.NormalizedEntryType()

	if batch.ID == 0 {
		query := `
			INSERT INTO production_batches (
				plan_id, entry_type, type_id, item_name, runs, units_output,
				total_material_cost, actual_production_cost, actual_sell_price,
				total_revenue, net_profit, roi, status, market_order_id,
				current_market_price, is_undercut, safe_relist_price,
				remaining_safe_margin, started_at, finished_at, listed_at, sold_at,
				created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24)
			RETURNING id, created_at`

		return r.pool.QueryRow(ctx, query,
			batch.PlanID, batch.EntryType, batch.TypeID, batch.ItemName, batch.Runs, batch.UnitsOutput,
			batch.TotalMaterialCost, batch.ActualProductionCost, batch.ActualSellPrice,
			batch.TotalRevenue, batch.NetProfit, batch.ROI, batch.Status, batch.MarketOrderID,
			batch.CurrentMarketPrice, batch.IsUndercut, batch.SafeRelistPrice,
			batch.RemainingSafeMargin, batch.StartedAt, batch.FinishedAt, batch.ListedAt, batch.SoldAt,
			now, now,
		).Scan(&batch.ID, &batch.CreatedAt)
	}

	query := `
		UPDATE production_batches SET
			plan_id = $1, entry_type = $2, type_id = $3, item_name = $4, runs = $5, units_output = $6,
			total_material_cost = $7, actual_production_cost = $8, actual_sell_price = $9,
			total_revenue = $10, net_profit = $11, roi = $12, status = $13,
			market_order_id = $14, current_market_price = $15, is_undercut = $16,
			safe_relist_price = $17, remaining_safe_margin = $18, started_at = $19,
			finished_at = $20, listed_at = $21, sold_at = $22, updated_at = $23
		WHERE id = $24`

	_, err := r.pool.Exec(ctx, query,
		batch.PlanID, batch.EntryType, batch.TypeID, batch.ItemName, batch.Runs, batch.UnitsOutput,
		batch.TotalMaterialCost, batch.ActualProductionCost, batch.ActualSellPrice,
		batch.TotalRevenue, batch.NetProfit, batch.ROI, batch.Status,
		batch.MarketOrderID, batch.CurrentMarketPrice, batch.IsUndercut,
		batch.SafeRelistPrice, batch.RemainingSafeMargin, batch.StartedAt,
		batch.FinishedAt, batch.ListedAt, batch.SoldAt, now, batch.ID,
	)
	return err
}

func (r *BatchRepo) UpdateBatchStatus(ctx context.Context, id int64, status domain.BatchStatus) error {
	now := time.Now()
	var query string
	var err error

	switch status {
	case domain.BatchStatusInProduction:
		query = `UPDATE production_batches SET status = $1, started_at = COALESCE(started_at, $2), updated_at = $2 WHERE id = $3`
	case domain.BatchStatusListed:
		query = `UPDATE production_batches SET status = $1, finished_at = COALESCE(finished_at, $2), listed_at = COALESCE(listed_at, $2), updated_at = $2 WHERE id = $3`
	case domain.BatchStatusSold:
		query = `UPDATE production_batches SET status = $1, sold_at = COALESCE(sold_at, $2), updated_at = $2 WHERE id = $3`
	default:
		query = `UPDATE production_batches SET status = $1, updated_at = $2 WHERE id = $3`
	}

	_, err = r.pool.Exec(ctx, query, status, now, id)
	return err
}

func (r *BatchRepo) DeleteBatch(ctx context.Context, id int64) error {
	query := `DELETE FROM production_batches WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, id)
	return err
}

func (r *BatchRepo) CreateBatchesFromPlan(ctx context.Context, planID int64, batches []domain.BatchRecord) error {
	if len(batches) == 0 {
		return nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("starting tx for plan batches: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO production_batches (
			plan_id, entry_type, type_id, item_name, runs, units_output,
			total_material_cost, actual_production_cost, actual_sell_price,
			total_revenue, net_profit, roi, status, market_order_id,
			current_market_price, is_undercut, safe_relist_price,
			remaining_safe_margin, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)`

	now := time.Now()
	for _, b := range batches {
		_, err := tx.Exec(ctx, query,
			planID, b.NormalizedEntryType(), b.TypeID, b.ItemName, b.Runs, b.UnitsOutput,
			b.TotalMaterialCost, b.ActualProductionCost, b.ActualSellPrice,
			b.TotalRevenue, b.NetProfit, b.ROI, domain.BatchStatusDraft, nil,
			b.CurrentMarketPrice, false, b.SafeRelistPrice,
			b.RemainingSafeMargin, now, now,
		)
		if err != nil {
			return fmt.Errorf("inserting batch from plan: %w", err)
		}
	}

	return tx.Commit(ctx)
}
