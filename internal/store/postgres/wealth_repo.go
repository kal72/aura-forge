package postgres

import (
	"context"
	"fmt"
	"time"

	"aura-forge/internal/domain"

	"github.com/jackc/pgx/v5/pgxpool"
)

// WealthRepo implements domain.NetWorthRepository backed by PostgreSQL.
type WealthRepo struct {
	pool *pgxpool.Pool
}

// NewWealthRepo creates a new PostgreSQL-backed NetWorthRepository.
func NewWealthRepo(pool *pgxpool.Pool) *WealthRepo {
	return &WealthRepo{pool: pool}
}

func (r *WealthRepo) ListSnapshots(ctx context.Context, limit int) ([]domain.NetWorthSnapshot, error) {
	if limit <= 0 || limit > 365 {
		limit = 90
	}
	// Fetch latest N snapshots, then order chronologically ascending for equity curve rendering
	rows, err := r.pool.Query(ctx, `
		SELECT id, TO_CHAR(snapshot_date, 'YYYY-MM-DD'), wallet_isk, sell_orders_isk, escrow_isk, inventory_isk, active_jobs_isk, total_net_worth, note, created_at, updated_at
		FROM (
			SELECT * FROM networth_snapshots
			ORDER BY snapshot_date DESC
			LIMIT $1
		) sub
		ORDER BY snapshot_date ASC
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("listing networth_snapshots: %w", err)
	}
	defer rows.Close()

	out := make([]domain.NetWorthSnapshot, 0)
	for rows.Next() {
		var s domain.NetWorthSnapshot
		if err := rows.Scan(
			&s.ID, &s.SnapshotDate, &s.WalletISK, &s.SellOrdersISK,
			&s.EscrowISK, &s.InventoryISK, &s.ActiveJobsISK, &s.TotalNetWorth,
			&s.Note, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning networth_snapshot: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *WealthRepo) UpsertSnapshot(ctx context.Context, s *domain.NetWorthSnapshot) error {
	if s.SnapshotDate == "" {
		s.SnapshotDate = time.Now().Format("2006-01-02")
	}
	s.ComputeTotal()

	err := r.pool.QueryRow(ctx, `
		INSERT INTO networth_snapshots (snapshot_date, wallet_isk, sell_orders_isk, escrow_isk, inventory_isk, active_jobs_isk, total_net_worth, note, created_at, updated_at)
		VALUES ($1::date, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
		ON CONFLICT (snapshot_date) DO UPDATE SET
			wallet_isk = EXCLUDED.wallet_isk,
			sell_orders_isk = EXCLUDED.sell_orders_isk,
			escrow_isk = EXCLUDED.escrow_isk,
			inventory_isk = EXCLUDED.inventory_isk,
			active_jobs_isk = EXCLUDED.active_jobs_isk,
			total_net_worth = EXCLUDED.total_net_worth,
			note = EXCLUDED.note,
			updated_at = NOW()
		RETURNING id, created_at, updated_at
	`, s.SnapshotDate, s.WalletISK, s.SellOrdersISK, s.EscrowISK, s.InventoryISK, s.ActiveJobsISK, s.TotalNetWorth, s.Note).Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upserting networth_snapshot: %w", err)
	}
	return nil
}

func (r *WealthRepo) DeleteSnapshot(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM networth_snapshots WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting networth_snapshot %d: %w", id, err)
	}
	return nil
}
