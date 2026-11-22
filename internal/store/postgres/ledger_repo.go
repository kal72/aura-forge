package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"aura-forge/internal/domain"

	"github.com/jackc/pgx/v5/pgxpool"
)

// LedgerRepo implements domain.TradeLedgerRepository on PostgreSQL.
type LedgerRepo struct {
	pool *pgxpool.Pool
}

func NewLedgerRepo(pool *pgxpool.Pool) *LedgerRepo {
	return &LedgerRepo{pool: pool}
}

func (r *LedgerRepo) UpsertTransactions(ctx context.Context, txs []domain.MarketTransaction) (int, int, error) {
	if len(txs) == 0 {
		return 0, 0, nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("starting ledger upsert tx: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO market_transactions (
			character_id, external_tx_id, dedup_hash, transacted_at, side,
			type_id, item_name, quantity, unit_price, total_isk, estimated_fee,
			location_id, hub_name, source, notes
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		ON CONFLICT (dedup_hash) DO NOTHING`

	inserted := 0
	skipped := 0
	for _, item := range txs {
		if item.DedupHash == "" {
			item.DedupHash = domain.ComputeTransactionDedupHash(item)
		}
		if item.TransactedAt.IsZero() {
			item.TransactedAt = time.Now().UTC()
		}
		if item.Source == "" {
			item.Source = domain.TransactionSourceManual
		}
		tag, err := tx.Exec(ctx, query,
			item.CharacterID, item.ExternalTxID, item.DedupHash, item.TransactedAt, string(item.Side),
			item.TypeID, item.ItemName, item.Quantity, item.UnitPrice, item.TotalISK, item.EstimatedFee,
			item.LocationID, item.HubName, string(item.Source), item.Notes,
		)
		if err != nil {
			return inserted, skipped, fmt.Errorf("inserting transaction %q: %w", item.ItemName, err)
		}
		if tag.RowsAffected() > 0 {
			inserted++
		} else {
			skipped++
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, 0, err
	}
	return inserted, skipped, nil
}

func (r *LedgerRepo) ListTransactions(ctx context.Context, periodDays int, sideFilter string, searchQuery string, limit int) ([]domain.MarketTransaction, error) {
	if limit <= 0 || limit > 2000 {
		limit = 500
	}

	var conds []string
	var args []any
	argIdx := 1

	if periodDays > 0 {
		conds = append(conds, fmt.Sprintf("transacted_at >= NOW() - (%d || ' days')::interval", periodDays))
	}
	sf := strings.ToUpper(strings.TrimSpace(sideFilter))
	if sf == "BUY" || sf == "SELL" {
		conds = append(conds, fmt.Sprintf("side = $%d", argIdx))
		args = append(args, sf)
		argIdx++
	}
	sq := strings.TrimSpace(searchQuery)
	if sq != "" {
		conds = append(conds, fmt.Sprintf("(LOWER(item_name) LIKE $%d OR LOWER(hub_name) LIKE $%d OR LOWER(notes) LIKE $%d)", argIdx, argIdx, argIdx))
		args = append(args, "%"+strings.ToLower(sq)+"%")
		argIdx++
	}

	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT id, character_id, external_tx_id, dedup_hash, transacted_at, side,
		       type_id, item_name, quantity, unit_price, total_isk, estimated_fee,
		       location_id, hub_name, source, notes, created_at
		FROM market_transactions
		%s
		ORDER BY transacted_at DESC, id DESC
		LIMIT $%d`, where, argIdx)
	args = append(args, limit)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing market transactions: %w", err)
	}
	defer rows.Close()

	var out []domain.MarketTransaction
	for rows.Next() {
		var tx domain.MarketTransaction
		var sideStr, srcStr string
		if err := rows.Scan(
			&tx.ID, &tx.CharacterID, &tx.ExternalTxID, &tx.DedupHash, &tx.TransactedAt, &sideStr,
			&tx.TypeID, &tx.ItemName, &tx.Quantity, &tx.UnitPrice, &tx.TotalISK, &tx.EstimatedFee,
			&tx.LocationID, &tx.HubName, &srcStr, &tx.Notes, &tx.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning market transaction: %w", err)
		}
		tx.Side = domain.TransactionSide(sideStr)
		tx.Source = domain.TransactionSource(srcStr)
		out = append(out, tx)
	}
	return out, nil
}

func (r *LedgerRepo) DeleteTransaction(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM market_transactions WHERE id = $1`, id)
	return err
}

func (r *LedgerRepo) UpsertFeeEntries(ctx context.Context, entries []domain.WalletFeeEntry) (int, error) {
	if len(entries) == 0 {
		return 0, nil
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO wallet_fee_entries (character_id, journal_id, recorded_at, ref_type, amount_isk, description)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (journal_id) DO NOTHING`

	inserted := 0
	for _, e := range entries {
		tag, err := tx.Exec(ctx, query, e.CharacterID, e.JournalID, e.RecordedAt, e.RefType, e.AmountISK, e.Description)
		if err != nil {
			return inserted, fmt.Errorf("inserting wallet fee entry: %w", err)
		}
		if tag.RowsAffected() > 0 {
			inserted++
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return inserted, nil
}

func (r *LedgerRepo) ListFeeEntries(ctx context.Context, periodDays int) ([]domain.WalletFeeEntry, error) {
	where := ""
	if periodDays > 0 {
		where = fmt.Sprintf("WHERE recorded_at >= NOW() - (%d || ' days')::interval", periodDays)
	}
	query := fmt.Sprintf(`
		SELECT id, character_id, journal_id, recorded_at, ref_type, amount_isk, description, created_at
		FROM wallet_fee_entries
		%s
		ORDER BY recorded_at DESC
		LIMIT 2000`, where)

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listing wallet fee entries: %w", err)
	}
	defer rows.Close()

	var out []domain.WalletFeeEntry
	for rows.Next() {
		var e domain.WalletFeeEntry
		if err := rows.Scan(&e.ID, &e.CharacterID, &e.JournalID, &e.RecordedAt, &e.RefType, &e.AmountISK, &e.Description, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

func (r *LedgerRepo) UpsertOrderHistory(ctx context.Context, orders []domain.CharacterOrderHistory) (int, error) {
	if len(orders) == 0 {
		return 0, nil
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO character_order_history (
			order_id, character_id, type_id, type_name, is_buy_order, price,
			volume_total, volume_remain, state, region_id, location_id, issued_at, duration_days, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,NOW())
		ON CONFLICT (order_id) DO UPDATE SET
			type_name = EXCLUDED.type_name,
			price = EXCLUDED.price,
			volume_remain = EXCLUDED.volume_remain,
			state = EXCLUDED.state,
			updated_at = NOW()`

	count := 0
	for _, o := range orders {
		if _, err := tx.Exec(ctx, query,
			o.OrderID, o.CharacterID, o.TypeID, o.TypeName, o.IsBuyOrder, o.Price,
			o.VolumeTotal, o.VolumeRemain, o.State, o.RegionID, o.LocationID, o.IssuedAt, o.DurationDays,
		); err != nil {
			return count, fmt.Errorf("upserting order history #%d: %w", o.OrderID, err)
		}
		count++
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *LedgerRepo) ListOrderHistory(ctx context.Context, characterID int64, limit int) ([]domain.CharacterOrderHistory, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var query string
	var args []any
	if characterID > 0 {
		query = `
			SELECT order_id, character_id, type_id, type_name, is_buy_order, price,
			       volume_total, volume_remain, state, region_id, location_id, issued_at, duration_days, updated_at
			FROM character_order_history
			WHERE character_id = $1
			ORDER BY issued_at DESC
			LIMIT $2`
		args = []any{characterID, limit}
	} else {
		query = `
			SELECT order_id, character_id, type_id, type_name, is_buy_order, price,
			       volume_total, volume_remain, state, region_id, location_id, issued_at, duration_days, updated_at
			FROM character_order_history
			ORDER BY issued_at DESC
			LIMIT $1`
		args = []any{limit}
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing order history: %w", err)
	}
	defer rows.Close()

	var out []domain.CharacterOrderHistory
	for rows.Next() {
		var o domain.CharacterOrderHistory
		if err := rows.Scan(
			&o.OrderID, &o.CharacterID, &o.TypeID, &o.TypeName, &o.IsBuyOrder, &o.Price,
			&o.VolumeTotal, &o.VolumeRemain, &o.State, &o.RegionID, &o.LocationID, &o.IssuedAt, &o.DurationDays, &o.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, nil
}
