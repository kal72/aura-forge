package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"aura-forge/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SDERepo implements domain.SDERepository backed by PostgreSQL SDE tables.
type SDERepo struct {
	pool *pgxpool.Pool
}

// NewSDERepo creates a new PostgreSQL-backed SDERepository.
func NewSDERepo(pool *pgxpool.Pool) *SDERepo {
	return &SDERepo{pool: pool}
}

func (r *SDERepo) SearchBlueprints(ctx context.Context, query, category string, limit int) ([]domain.SDEBlueprint, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	query = strings.TrimSpace(query)
	category = strings.TrimSpace(category)

	sqlQuery := `
		SELECT blueprint_type_id, name, product_type_id, product_name, product_qty_per_run, base_time_seconds, tech_level, category
		FROM sde_blueprints
		WHERE ($1 = '' OR LOWER(name) LIKE '%' || LOWER($1) || '%' OR LOWER(product_name) LIKE '%' || LOWER($1) || '%')
		  AND ($2 = '' OR LOWER(category) = LOWER($2))
		ORDER BY category ASC, product_name ASC
		LIMIT $3
	`
	rows, err := r.pool.Query(ctx, sqlQuery, query, category, limit)
	if err != nil {
		return nil, fmt.Errorf("searching sde_blueprints: %w", err)
	}
	defer rows.Close()

	out := make([]domain.SDEBlueprint, 0)
	for rows.Next() {
		var bp domain.SDEBlueprint
		if err := rows.Scan(
			&bp.BlueprintTypeID, &bp.Name, &bp.ProductTypeID, &bp.ProductName,
			&bp.ProductQtyPerRun, &bp.BaseTimeSeconds, &bp.TechLevel, &bp.Category,
		); err != nil {
			return nil, fmt.Errorf("scanning sde_blueprint: %w", err)
		}
		out = append(out, bp)
	}
	return out, rows.Err()
}

func (r *SDERepo) GetBlueprintByID(ctx context.Context, blueprintTypeID int32) (*domain.SDEBlueprint, error) {
	var bp domain.SDEBlueprint
	err := r.pool.QueryRow(ctx, `
		SELECT blueprint_type_id, name, product_type_id, product_name, product_qty_per_run, base_time_seconds, tech_level, category
		FROM sde_blueprints
		WHERE blueprint_type_id = $1
	`, blueprintTypeID).Scan(
		&bp.BlueprintTypeID, &bp.Name, &bp.ProductTypeID, &bp.ProductName,
		&bp.ProductQtyPerRun, &bp.BaseTimeSeconds, &bp.TechLevel, &bp.Category,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting sde_blueprint %d: %w", blueprintTypeID, err)
	}
	return &bp, nil
}

func (r *SDERepo) GetBlueprintByProductID(ctx context.Context, productTypeID int32) (*domain.SDEBlueprint, error) {
	var bp domain.SDEBlueprint
	err := r.pool.QueryRow(ctx, `
		SELECT blueprint_type_id, name, product_type_id, product_name, product_qty_per_run, base_time_seconds, tech_level, category
		FROM sde_blueprints
		WHERE product_type_id = $1
		LIMIT 1
	`, productTypeID).Scan(
		&bp.BlueprintTypeID, &bp.Name, &bp.ProductTypeID, &bp.ProductName,
		&bp.ProductQtyPerRun, &bp.BaseTimeSeconds, &bp.TechLevel, &bp.Category,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting sde_blueprint by product %d: %w", productTypeID, err)
	}
	return &bp, nil
}

func (r *SDERepo) GetBlueprintMaterials(ctx context.Context, blueprintTypeID int32) ([]domain.SDEMaterialRequirement, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT m.material_type_id, t.name, m.base_quantity, t.volume_m3
		FROM sde_blueprint_materials m
		JOIN sde_types t ON t.type_id = m.material_type_id
		WHERE m.blueprint_type_id = $1
		ORDER BY m.base_quantity DESC, t.name ASC
	`, blueprintTypeID)
	if err != nil {
		return nil, fmt.Errorf("getting materials for blueprint %d: %w", blueprintTypeID, err)
	}
	defer rows.Close()

	out := make([]domain.SDEMaterialRequirement, 0)
	for rows.Next() {
		var m domain.SDEMaterialRequirement
		if err := rows.Scan(&m.MaterialTypeID, &m.MaterialName, &m.BaseQuantity, &m.VolumeM3); err != nil {
			return nil, fmt.Errorf("scanning sde_blueprint_material: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *SDERepo) UpsertSDEData(ctx context.Context, types []domain.SDEType, blueprints []domain.SDEBlueprint, materials map[int32][]domain.SDEMaterialRequirement) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning SDE upsert transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, t := range types {
		if _, err := tx.Exec(ctx, `
			INSERT INTO sde_types (type_id, name, group_name, category, volume_m3, portion_size)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (type_id) DO UPDATE SET
				name = EXCLUDED.name,
				group_name = EXCLUDED.group_name,
				category = EXCLUDED.category,
				volume_m3 = EXCLUDED.volume_m3,
				portion_size = EXCLUDED.portion_size
		`, t.TypeID, t.Name, t.GroupName, t.Category, t.VolumeM3, t.PortionSize); err != nil {
			return fmt.Errorf("upserting sde_type %d: %w", t.TypeID, err)
		}
	}

	for _, bp := range blueprints {
		if _, err := tx.Exec(ctx, `
			INSERT INTO sde_blueprints (blueprint_type_id, name, product_type_id, product_name, product_qty_per_run, base_time_seconds, tech_level, category)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (blueprint_type_id) DO UPDATE SET
				name = EXCLUDED.name,
				product_type_id = EXCLUDED.product_type_id,
				product_name = EXCLUDED.product_name,
				product_qty_per_run = EXCLUDED.product_qty_per_run,
				base_time_seconds = EXCLUDED.base_time_seconds,
				tech_level = EXCLUDED.tech_level,
				category = EXCLUDED.category
		`, bp.BlueprintTypeID, bp.Name, bp.ProductTypeID, bp.ProductName, bp.ProductQtyPerRun, bp.BaseTimeSeconds, bp.TechLevel, bp.Category); err != nil {
			return fmt.Errorf("upserting sde_blueprint %d: %w", bp.BlueprintTypeID, err)
		}
	}

	for bpID, mats := range materials {
		if _, err := tx.Exec(ctx, `DELETE FROM sde_blueprint_materials WHERE blueprint_type_id = $1`, bpID); err != nil {
			return fmt.Errorf("clearing materials for blueprint %d: %w", bpID, err)
		}
		for _, m := range mats {
			if _, err := tx.Exec(ctx, `
				INSERT INTO sde_blueprint_materials (blueprint_type_id, material_type_id, base_quantity)
				VALUES ($1, $2, $3)
				ON CONFLICT (blueprint_type_id, material_type_id) DO UPDATE SET
					base_quantity = EXCLUDED.base_quantity
			`, bpID, m.MaterialTypeID, m.BaseQuantity); err != nil {
				return fmt.Errorf("upserting material %d for blueprint %d: %w", m.MaterialTypeID, bpID, err)
			}
		}
	}

	return tx.Commit(ctx)
}
