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

type SettingsRepo struct {
	pool *pgxpool.Pool
}

func NewSettingsRepo(pool *pgxpool.Pool) *SettingsRepo {
	return &SettingsRepo{pool: pool}
}

func (r *SettingsRepo) GetSettings(ctx context.Context) (*domain.UserSettings, error) {
	query := `
		SELECT id, starting_capital, broker_fee_percent, sales_tax_percent, relist_fee_percent,
		       manufacturing_slots, science_slots, system_cost_index, freight_cost_per_m3,
		       facility_tax_percent, auto_price_sync_enabled, auto_price_sync_interval_minutes,
		       default_trade_hub, updated_at
		FROM user_settings
		WHERE id = 1`

	var s domain.UserSettings
	err := r.pool.QueryRow(ctx, query).Scan(
		&s.ID, &s.StartingCapital, &s.BrokerFeePercent, &s.SalesTaxPercent,
		&s.RelistFeePercent, &s.ManufacturingSlots, &s.ScienceSlots,
		&s.SystemCostIndex, &s.FreightCostPerM3, &s.FacilityTaxPercent,
		&s.AutoPriceSyncEnabled, &s.AutoPriceSyncIntervalMinutes,
		&s.DefaultTradeHub, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			defaults := domain.DefaultUserSettings()
			_ = r.SaveSettings(ctx, &defaults)
			return &defaults, nil
		}
		return nil, fmt.Errorf("failed to get user settings: %w", err)
	}
	s.AutoPriceSyncIntervalMinutes = domain.ClampPriceSyncInterval(s.AutoPriceSyncIntervalMinutes)
	s.DefaultTradeHub = domain.LookupTradeHub(s.DefaultTradeHub).Key

	return &s, nil
}

func (r *SettingsRepo) SaveSettings(ctx context.Context, s *domain.UserSettings) error {
	query := `
		INSERT INTO user_settings (
			id, starting_capital, broker_fee_percent, sales_tax_percent, relist_fee_percent,
			manufacturing_slots, science_slots, system_cost_index, freight_cost_per_m3,
			facility_tax_percent, auto_price_sync_enabled, auto_price_sync_interval_minutes,
			default_trade_hub, updated_at
		) VALUES (1, $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (id) DO UPDATE SET
			starting_capital = EXCLUDED.starting_capital,
			broker_fee_percent = EXCLUDED.broker_fee_percent,
			sales_tax_percent = EXCLUDED.sales_tax_percent,
			relist_fee_percent = EXCLUDED.relist_fee_percent,
			manufacturing_slots = EXCLUDED.manufacturing_slots,
			science_slots = EXCLUDED.science_slots,
			system_cost_index = EXCLUDED.system_cost_index,
			freight_cost_per_m3 = EXCLUDED.freight_cost_per_m3,
			facility_tax_percent = EXCLUDED.facility_tax_percent,
			auto_price_sync_enabled = EXCLUDED.auto_price_sync_enabled,
			auto_price_sync_interval_minutes = EXCLUDED.auto_price_sync_interval_minutes,
			default_trade_hub = EXCLUDED.default_trade_hub,
			updated_at = EXCLUDED.updated_at`

	now := time.Now()
	s.ID = 1
	s.AutoPriceSyncIntervalMinutes = domain.ClampPriceSyncInterval(s.AutoPriceSyncIntervalMinutes)
	s.DefaultTradeHub = domain.LookupTradeHub(s.DefaultTradeHub).Key
	s.UpdatedAt = now

	_, err := r.pool.Exec(ctx, query,
		s.StartingCapital, s.BrokerFeePercent, s.SalesTaxPercent, s.RelistFeePercent,
		s.ManufacturingSlots, s.ScienceSlots, s.SystemCostIndex, s.FreightCostPerM3,
		s.FacilityTaxPercent, s.AutoPriceSyncEnabled, s.AutoPriceSyncIntervalMinutes,
		s.DefaultTradeHub, now,
	)
	return err
}
