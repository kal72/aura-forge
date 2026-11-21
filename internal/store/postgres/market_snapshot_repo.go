package postgres

import (
	"context"
	"fmt"
	"time"

	"aura-forge/internal/domain"

	"github.com/jackc/pgx/v5/pgxpool"
)

// MarketSnapshotRepo implements domain.GlobalPriceSnapshotRepository in PostgreSQL.
type MarketSnapshotRepo struct {
	pool *pgxpool.Pool
}

// NewMarketSnapshotRepo creates a new PostgreSQL-backed GlobalPriceSnapshotRepository.
func NewMarketSnapshotRepo(pool *pgxpool.Pool) *MarketSnapshotRepo {
	return &MarketSnapshotRepo{pool: pool}
}

// UpsertDailySnapshot bulk-upserts ~15,000 global market prices for a UTC date using UNNEST in a single statement.
func (r *MarketSnapshotRepo) UpsertDailySnapshot(ctx context.Context, snapshotDate time.Time, prices []domain.GlobalMarketPrice) (int, error) {
	if len(prices) == 0 {
		return 0, nil
	}

	day := snapshotDate.UTC().Format("2006-01-02")
	typeIDs := make([]int32, 0, len(prices))
	avgs := make([]float64, 0, len(prices))
	adjs := make([]float64, 0, len(prices))

	for _, p := range prices {
		if p.TypeID <= 0 || (p.AveragePrice <= 0 && p.AdjustedPrice <= 0) {
			continue
		}
		typeIDs = append(typeIDs, p.TypeID)
		avgs = append(avgs, p.AveragePrice)
		adjs = append(adjs, p.AdjustedPrice)
	}
	if len(typeIDs) == 0 {
		return 0, nil
	}

	const query = `
		INSERT INTO global_market_price_snapshots (snapshot_date, type_id, average_price, adjusted_price, updated_at)
		SELECT $1::date, t.type_id, t.avg_price, t.adj_price, NOW()
		FROM UNNEST($2::int[], $3::numeric[], $4::numeric[]) AS t(type_id, avg_price, adj_price)
		ON CONFLICT (snapshot_date, type_id) DO UPDATE SET
			average_price = EXCLUDED.average_price,
			adjusted_price = EXCLUDED.adjusted_price,
			updated_at = NOW()
	`
	if _, err := r.pool.Exec(ctx, query, day, typeIDs, avgs, adjs); err != nil {
		return 0, fmt.Errorf("bulk upserting global market price snapshot (%d rows): %w", len(typeIDs), err)
	}
	return len(typeIDs), nil
}

// ListTopPriceMovers compares the latest snapshot date against the immediately preceding snapshot date
// and returns the Top N rising items (filtering out penny noise below minPrevPrice and extreme manipulation spikes).
func (r *MarketSnapshotRepo) ListTopPriceMovers(ctx context.Context, limit int, minPrevPrice float64) ([]domain.GlobalPriceMover, error) {
	if limit <= 0 || limit > 50 {
		limit = 8
	}
	if minPrevPrice <= 0 {
		minPrevPrice = 1000
	}

	const query = `
		WITH recent_dates AS (
			SELECT DISTINCT snapshot_date
			FROM global_market_price_snapshots
			ORDER BY snapshot_date DESC
			LIMIT 2
		),
		date_bounds AS (
			SELECT MAX(snapshot_date) AS latest_date, MIN(snapshot_date) AS prev_date, COUNT(*) AS cnt
			FROM recent_dates
		)
		SELECT
			cur.type_id,
			COALESCE(st.name, '') AS item_name,
			COALESCE(st.category, 'Market') AS item_category,
			cur.average_price::float8 AS current_avg,
			prev.average_price::float8 AS previous_avg,
			ROUND(((cur.average_price - prev.average_price) / prev.average_price) * 100.0, 2)::float8 AS change_pct,
			cur.snapshot_date
		FROM date_bounds db
		JOIN global_market_price_snapshots cur ON cur.snapshot_date = db.latest_date
		JOIN global_market_price_snapshots prev ON prev.snapshot_date = db.prev_date AND prev.type_id = cur.type_id
		LEFT JOIN sde_types st ON st.type_id = cur.type_id
		WHERE db.cnt = 2
		  AND prev.average_price >= $1
		  AND cur.average_price > prev.average_price
		  AND ((cur.average_price - prev.average_price) / prev.average_price) <= 5.0
		ORDER BY change_pct DESC, cur.average_price DESC
		LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, minPrevPrice, limit)
	if err != nil {
		return nil, fmt.Errorf("querying top global price movers: %w", err)
	}
	defer rows.Close()

	out := make([]domain.GlobalPriceMover, 0, limit)
	for rows.Next() {
		var m domain.GlobalPriceMover
		if err := rows.Scan(
			&m.TypeID, &m.Name, &m.Category,
			&m.CurrentAvg, &m.PreviousAvg, &m.ChangePercent, &m.SnapshotDate,
		); err != nil {
			return nil, fmt.Errorf("scanning global price mover: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// GetSnapshotStats returns how many days of snapshots exist and how many items are in the latest snapshot.
func (r *MarketSnapshotRepo) GetSnapshotStats(ctx context.Context) (*domain.GlobalSnapshotStats, error) {
	var stats domain.GlobalSnapshotStats
	err := r.pool.QueryRow(ctx, `
		SELECT
			COUNT(DISTINCT snapshot_date)::int,
			MAX(snapshot_date),
			MAX(updated_at)
		FROM global_market_price_snapshots
	`).Scan(&stats.DaysAvailable, &stats.LatestDate, &stats.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("reading global snapshot stats: %w", err)
	}
	if stats.LatestDate != nil {
		_ = r.pool.QueryRow(ctx, `
			SELECT COUNT(*)::int
			FROM global_market_price_snapshots
			WHERE snapshot_date = $1
		`, *stats.LatestDate).Scan(&stats.LatestItems)
	}
	return &stats, nil
}

// PruneOldSnapshots removes snapshots older than keepDays to keep database storage compact.
func (r *MarketSnapshotRepo) PruneOldSnapshots(ctx context.Context, keepDays int) error {
	if keepDays < 2 {
		keepDays = 14
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -keepDays).Format("2006-01-02")
	_, err := r.pool.Exec(ctx, `DELETE FROM global_market_price_snapshots WHERE snapshot_date < $1::date`, cutoff)
	return err
}
