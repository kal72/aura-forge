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

type PlanRepo struct {
	pool *pgxpool.Pool
}

func NewPlanRepo(pool *pgxpool.Pool) *PlanRepo {
	return &PlanRepo{pool: pool}
}

func (r *PlanRepo) ListPlans(ctx context.Context) ([]domain.WeeklyPlan, error) {
	query := `
		SELECT id, title, week_number, starting_cash, status,
		       manufacturing_slots_total, manufacturing_slots_used,
		       science_slots_total, science_slots_used,
		       days, items, total_estimated_cost, total_estimated_revenue,
		       total_estimated_profit, projected_roi, created_at, updated_at
		FROM weekly_plans
		ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query plans: %w", err)
	}
	defer rows.Close()

	var plans []domain.WeeklyPlan
	for rows.Next() {
		var p domain.WeeklyPlan
		var daysJSON, itemsJSON []byte

		err := rows.Scan(
			&p.ID, &p.Title, &p.WeekNumber, &p.StartingCash, &p.Status,
			&p.ManufacturingSlotsTotal, &p.ManufacturingSlotsUsed,
			&p.ScienceSlotsTotal, &p.ScienceSlotsUsed,
			&daysJSON, &itemsJSON,
			&p.TotalEstimatedCost, &p.TotalEstimatedRevenue,
			&p.TotalEstimatedProfit, &p.ProjectedROI,
			&p.CreatedAt, &p.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan plan: %w", err)
		}

		if len(daysJSON) > 0 {
			_ = json.Unmarshal(daysJSON, &p.Days)
		}
		if len(itemsJSON) > 0 {
			_ = json.Unmarshal(itemsJSON, &p.Items)
		}

		plans = append(plans, p)
	}

	return plans, nil
}

func (r *PlanRepo) GetPlanByID(ctx context.Context, id int64) (*domain.WeeklyPlan, error) {
	query := `
		SELECT id, title, week_number, starting_cash, status,
		       manufacturing_slots_total, manufacturing_slots_used,
		       science_slots_total, science_slots_used,
		       days, items, total_estimated_cost, total_estimated_revenue,
		       total_estimated_profit, projected_roi, created_at, updated_at
		FROM weekly_plans
		WHERE id = $1`

	var p domain.WeeklyPlan
	var daysJSON, itemsJSON []byte

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&p.ID, &p.Title, &p.WeekNumber, &p.StartingCash, &p.Status,
		&p.ManufacturingSlotsTotal, &p.ManufacturingSlotsUsed,
		&p.ScienceSlotsTotal, &p.ScienceSlotsUsed,
		&daysJSON, &itemsJSON,
		&p.TotalEstimatedCost, &p.TotalEstimatedRevenue,
		&p.TotalEstimatedProfit, &p.ProjectedROI,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get plan: %w", err)
	}

	if len(daysJSON) > 0 {
		_ = json.Unmarshal(daysJSON, &p.Days)
	}
	if len(itemsJSON) > 0 {
		_ = json.Unmarshal(itemsJSON, &p.Items)
	}

	return &p, nil
}

func (r *PlanRepo) GetActivePlan(ctx context.Context) (*domain.WeeklyPlan, error) {
	query := `
		SELECT id, title, week_number, starting_cash, status,
		       manufacturing_slots_total, manufacturing_slots_used,
		       science_slots_total, science_slots_used,
		       days, items, total_estimated_cost, total_estimated_revenue,
		       total_estimated_profit, projected_roi, created_at, updated_at
		FROM weekly_plans
		WHERE status = 'ACTIVE'
		ORDER BY updated_at DESC
		LIMIT 1`

	var p domain.WeeklyPlan
	var daysJSON, itemsJSON []byte

	err := r.pool.QueryRow(ctx, query).Scan(
		&p.ID, &p.Title, &p.WeekNumber, &p.StartingCash, &p.Status,
		&p.ManufacturingSlotsTotal, &p.ManufacturingSlotsUsed,
		&p.ScienceSlotsTotal, &p.ScienceSlotsUsed,
		&daysJSON, &itemsJSON,
		&p.TotalEstimatedCost, &p.TotalEstimatedRevenue,
		&p.TotalEstimatedProfit, &p.ProjectedROI,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get active plan: %w", err)
	}

	if len(daysJSON) > 0 {
		_ = json.Unmarshal(daysJSON, &p.Days)
	}
	if len(itemsJSON) > 0 {
		_ = json.Unmarshal(itemsJSON, &p.Items)
	}

	return &p, nil
}

func (r *PlanRepo) SavePlan(ctx context.Context, plan *domain.WeeklyPlan) error {
	daysJSON, err := json.Marshal(plan.Days)
	if err != nil {
		daysJSON = []byte("[]")
	}
	itemsJSON, err := json.Marshal(plan.Items)
	if err != nil {
		itemsJSON = []byte("[]")
	}

	now := time.Now()
	plan.UpdatedAt = now

	if plan.ID == 0 {
		query := `
			INSERT INTO weekly_plans (
				title, week_number, starting_cash, status,
				manufacturing_slots_total, manufacturing_slots_used,
				science_slots_total, science_slots_used,
				days, items, total_estimated_cost, total_estimated_revenue,
				total_estimated_profit, projected_roi, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
			RETURNING id, created_at`

		return r.pool.QueryRow(ctx, query,
			plan.Title, plan.WeekNumber, plan.StartingCash, plan.Status,
			plan.ManufacturingSlotsTotal, plan.ManufacturingSlotsUsed,
			plan.ScienceSlotsTotal, plan.ScienceSlotsUsed,
			daysJSON, itemsJSON, plan.TotalEstimatedCost, plan.TotalEstimatedRevenue,
			plan.TotalEstimatedProfit, plan.ProjectedROI, now, now,
		).Scan(&plan.ID, &plan.CreatedAt)
	}

	query := `
		UPDATE weekly_plans SET
			title = $1, week_number = $2, starting_cash = $3, status = $4,
			manufacturing_slots_total = $5, manufacturing_slots_used = $6,
			science_slots_total = $7, science_slots_used = $8,
			days = $9, items = $10, total_estimated_cost = $11, total_estimated_revenue = $12,
			total_estimated_profit = $13, projected_roi = $14, updated_at = $15
		WHERE id = $16`

	_, err = r.pool.Exec(ctx, query,
		plan.Title, plan.WeekNumber, plan.StartingCash, plan.Status,
		plan.ManufacturingSlotsTotal, plan.ManufacturingSlotsUsed,
		plan.ScienceSlotsTotal, plan.ScienceSlotsUsed,
		daysJSON, itemsJSON, plan.TotalEstimatedCost, plan.TotalEstimatedRevenue,
		plan.TotalEstimatedProfit, plan.ProjectedROI, now, plan.ID,
	)
	return err
}

func (r *PlanRepo) DeletePlan(ctx context.Context, id int64) error {
	query := `DELETE FROM weekly_plans WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, id)
	return err
}
