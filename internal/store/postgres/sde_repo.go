package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

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
		SELECT blueprint_type_id, name, product_type_id, product_name, product_qty_per_run, base_time_seconds, tech_level, category, activity_type
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
			&bp.ProductQtyPerRun, &bp.BaseTimeSeconds, &bp.TechLevel, &bp.Category, &bp.ActivityType,
		); err != nil {
			return nil, fmt.Errorf("scanning sde_blueprint: %w", err)
		}
		out = append(out, bp)
	}
	return out, rows.Err()
}

// GetTypeNamesByIDs resolves a batch of EVE TypeIDs to human-readable names from local sde_types and sde_blueprints.
func (r *SDERepo) GetTypeNamesByIDs(ctx context.Context, typeIDs []int32) (map[int32]string, error) {
	out := make(map[int32]string, len(typeIDs))
	if len(typeIDs) == 0 {
		return out, nil
	}

	rows, err := r.pool.Query(ctx, `SELECT type_id, name FROM sde_types WHERE type_id = ANY($1)`, typeIDs)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id int32
			var name string
			if err := rows.Scan(&id, &name); err == nil && name != "" {
				out[id] = name
			}
		}
	}

	bpRows, err := r.pool.Query(ctx, `SELECT blueprint_type_id, name FROM sde_blueprints WHERE blueprint_type_id = ANY($1)`, typeIDs)
	if err == nil {
		defer bpRows.Close()
		for bpRows.Next() {
			var id int32
			var name string
			if err := bpRows.Scan(&id, &name); err == nil && name != "" {
				if _, exists := out[id]; !exists {
					out[id] = name
				}
			}
		}
	}

	return out, nil
}

// SearchTypes searches sde_types for instant local Marketplace autocomplete (0 ESI calls).
// Prioritizes exact/prefix matches and non-blueprint tradable items first.
func (r *SDERepo) SearchTypes(ctx context.Context, query string, limit int) ([]domain.SDEType, error) {
	query = strings.TrimSpace(query)
	if len(query) < 2 {
		return []domain.SDEType{}, nil
	}
	if limit <= 0 || limit > 30 {
		limit = 12
	}

	rows, err := r.pool.Query(ctx, `
		SELECT type_id, name, group_name, category, volume_m3, portion_size
		FROM sde_types
		WHERE LOWER(name) LIKE '%' || LOWER($1) || '%'
		ORDER BY
			CASE WHEN LOWER(name) = LOWER($1) THEN 0
			     WHEN LOWER(name) LIKE LOWER($1) || '%' THEN 1
			     ELSE 2 END ASC,
			CASE WHEN LOWER(name) LIKE '%blueprint%' THEN 1 ELSE 0 END ASC,
			LENGTH(name) ASC,
			name ASC
		LIMIT $2
	`, query, limit)
	if err != nil {
		return nil, fmt.Errorf("searching sde_types: %w", err)
	}
	defer rows.Close()

	out := make([]domain.SDEType, 0, limit)
	for rows.Next() {
		var t domain.SDEType
		if err := rows.Scan(&t.TypeID, &t.Name, &t.GroupName, &t.Category, &t.VolumeM3, &t.PortionSize); err != nil {
			return nil, fmt.Errorf("scanning sde_type: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// BrowseTypesPaged queries sde_types with optional search query, category, and group_name filters,
// returning paginated results along with category counts and sub-group facets (0 ESI calls).
func (r *SDERepo) BrowseTypesPaged(ctx context.Context, query, category, groupName string, page, pageSize int) (*domain.MarketTypePageResult, error) {
	query = strings.TrimSpace(query)
	category = strings.TrimSpace(category)
	groupName = strings.TrimSpace(groupName)
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 60 {
		pageSize = 24
	}

	// 1. Total matching items
	var total int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM sde_types
		WHERE ($1 = '' OR LOWER(name) LIKE '%' || LOWER($1) || '%' OR LOWER(group_name) LIKE '%' || LOWER($1) || '%')
		  AND ($2 = '' OR LOWER(category) = LOWER($2))
		  AND ($3 = '' OR LOWER(group_name) = LOWER($3))
	`, query, category, groupName).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("counting sde_types: %w", err)
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))
	if totalPages < 1 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}
	offset := (page - 1) * pageSize

	// 2. Paginated item rows
	rows, err := r.pool.Query(ctx, `
		SELECT type_id, name, group_name, category, volume_m3, portion_size
		FROM sde_types
		WHERE ($1 = '' OR LOWER(name) LIKE '%' || LOWER($1) || '%' OR LOWER(group_name) LIKE '%' || LOWER($1) || '%')
		  AND ($2 = '' OR LOWER(category) = LOWER($2))
		  AND ($3 = '' OR LOWER(group_name) = LOWER($3))
		ORDER BY
			CASE WHEN $1 <> '' AND LOWER(name) LIKE LOWER($1) || '%' THEN 0 ELSE 1 END ASC,
			name ASC
		LIMIT $4 OFFSET $5
	`, query, category, groupName, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("browsing sde_types: %w", err)
	}
	defer rows.Close()

	items := make([]domain.SDEType, 0, pageSize)
	for rows.Next() {
		var t domain.SDEType
		if err := rows.Scan(&t.TypeID, &t.Name, &t.GroupName, &t.Category, &t.VolumeM3, &t.PortionSize); err != nil {
			return nil, fmt.Errorf("scanning sde_type browse row: %w", err)
		}
		items = append(items, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 3. Category facet counts across all sde_types
	catRows, err := r.pool.Query(ctx, `
		SELECT category, COUNT(*) AS cnt
		FROM sde_types
		WHERE category <> ''
		GROUP BY category
		ORDER BY cnt DESC, category ASC
	`)
	categories := make([]domain.CategoryCount, 0, 16)
	if err == nil {
		defer catRows.Close()
		for catRows.Next() {
			var cc domain.CategoryCount
			if catRows.Scan(&cc.Category, &cc.Count) == nil {
				categories = append(categories, cc)
			}
		}
	}

	// 4. Sub-group facet list when a specific category is selected
	groups := make([]string, 0, 32)
	if category != "" {
		gRows, gErr := r.pool.Query(ctx, `
			SELECT DISTINCT group_name
			FROM sde_types
			WHERE LOWER(category) = LOWER($1) AND group_name <> ''
			ORDER BY group_name ASC
			LIMIT 200
		`, category)
		if gErr == nil {
			defer gRows.Close()
			for gRows.Next() {
				var g string
				if gRows.Scan(&g) == nil {
					groups = append(groups, g)
				}
			}
		}
	}

	return &domain.MarketTypePageResult{
		Items:      items,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
		Categories: categories,
		Groups:     groups,
	}, nil
}

func (r *SDERepo) GetBlueprintByID(ctx context.Context, blueprintTypeID int32) (*domain.SDEBlueprint, error) {
	var bp domain.SDEBlueprint
	err := r.pool.QueryRow(ctx, `
		SELECT blueprint_type_id, name, product_type_id, product_name, product_qty_per_run, base_time_seconds, tech_level, category, activity_type
		FROM sde_blueprints
		WHERE blueprint_type_id = $1
	`, blueprintTypeID).Scan(
		&bp.BlueprintTypeID, &bp.Name, &bp.ProductTypeID, &bp.ProductName,
		&bp.ProductQtyPerRun, &bp.BaseTimeSeconds, &bp.TechLevel, &bp.Category, &bp.ActivityType,
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
		SELECT blueprint_type_id, name, product_type_id, product_name, product_qty_per_run, base_time_seconds, tech_level, category, activity_type
		FROM sde_blueprints
		WHERE product_type_id = $1
		LIMIT 1
	`, productTypeID).Scan(
		&bp.BlueprintTypeID, &bp.Name, &bp.ProductTypeID, &bp.ProductName,
		&bp.ProductQtyPerRun, &bp.BaseTimeSeconds, &bp.TechLevel, &bp.Category, &bp.ActivityType,
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

	const batchSize = 5000

	// 1. Bulk upsert sde_types via UNNEST
	for start := 0; start < len(types); start += batchSize {
		end := start + batchSize
		if end > len(types) {
			end = len(types)
		}
		chunk := types[start:end]
		ids := make([]int32, len(chunk))
		names := make([]string, len(chunk))
		groups := make([]string, len(chunk))
		cats := make([]string, len(chunk))
		vols := make([]float64, len(chunk))
		portions := make([]int32, len(chunk))
		traits := make([]string, len(chunk))
		for i, t := range chunk {
			ids[i] = t.TypeID
			names[i] = t.Name
			groups[i] = t.GroupName
			cats[i] = t.Category
			vols[i] = t.VolumeM3
			portions[i] = int32(t.PortionSize)
			traits[i] = t.TraitsJSON
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO sde_types (type_id, name, group_name, category, volume_m3, portion_size, traits_json)
			SELECT * FROM UNNEST($1::int[], $2::varchar[], $3::varchar[], $4::varchar[], $5::double precision[], $6::int[], $7::text[])
			ON CONFLICT (type_id) DO UPDATE SET
				name = EXCLUDED.name,
				group_name = EXCLUDED.group_name,
				category = EXCLUDED.category,
				volume_m3 = EXCLUDED.volume_m3,
				portion_size = EXCLUDED.portion_size,
				traits_json = CASE WHEN EXCLUDED.traits_json <> '' THEN EXCLUDED.traits_json ELSE sde_types.traits_json END
		`, ids, names, groups, cats, vols, portions, traits); err != nil {
			return fmt.Errorf("bulk upserting sde_types batch %d-%d: %w", start, end, err)
		}
	}

	// 2. Bulk upsert sde_blueprints via UNNEST
	for start := 0; start < len(blueprints); start += batchSize {
		end := start + batchSize
		if end > len(blueprints) {
			end = len(blueprints)
		}
		chunk := blueprints[start:end]
		bpIDs := make([]int32, len(chunk))
		bpNames := make([]string, len(chunk))
		prodIDs := make([]int32, len(chunk))
		prodNames := make([]string, len(chunk))
		qtys := make([]int32, len(chunk))
		times := make([]int32, len(chunk))
		techs := make([]int32, len(chunk))
		cats := make([]string, len(chunk))
		acts := make([]string, len(chunk))
		for i, bp := range chunk {
			bpIDs[i] = bp.BlueprintTypeID
			bpNames[i] = bp.Name
			prodIDs[i] = bp.ProductTypeID
			prodNames[i] = bp.ProductName
			qtys[i] = int32(bp.ProductQtyPerRun)
			times[i] = int32(bp.BaseTimeSeconds)
			techs[i] = int32(bp.TechLevel)
			cats[i] = bp.Category
			act := strings.TrimSpace(bp.ActivityType)
			if act == "" {
				if strings.EqualFold(bp.Category, "Reactions") {
					act = "reaction"
				} else {
					act = "manufacturing"
				}
			}
			acts[i] = act
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO sde_blueprints (blueprint_type_id, name, product_type_id, product_name, product_qty_per_run, base_time_seconds, tech_level, category, activity_type)
			SELECT * FROM UNNEST($1::int[], $2::varchar[], $3::int[], $4::varchar[], $5::int[], $6::int[], $7::int[], $8::varchar[], $9::varchar[])
			ON CONFLICT (blueprint_type_id) DO UPDATE SET
				name = EXCLUDED.name,
				product_type_id = EXCLUDED.product_type_id,
				product_name = EXCLUDED.product_name,
				product_qty_per_run = EXCLUDED.product_qty_per_run,
				base_time_seconds = EXCLUDED.base_time_seconds,
				tech_level = EXCLUDED.tech_level,
				category = EXCLUDED.category,
				activity_type = EXCLUDED.activity_type
		`, bpIDs, bpNames, prodIDs, prodNames, qtys, times, techs, cats, acts); err != nil {
			return fmt.Errorf("bulk upserting sde_blueprints batch %d-%d: %w", start, end, err)
		}
	}

	// 3. Clear & bulk upsert sde_blueprint_materials for touched blueprints
	if len(materials) > 0 {
		touchedBPs := make([]int32, 0, len(materials))
		flatBP := make([]int32, 0, len(materials)*5)
		flatMat := make([]int32, 0, len(materials)*5)
		flatQty := make([]int64, 0, len(materials)*5)
		for bpID, mats := range materials {
			touchedBPs = append(touchedBPs, bpID)
			for _, m := range mats {
				if m.MaterialTypeID <= 0 || m.BaseQuantity <= 0 {
					continue
				}
				flatBP = append(flatBP, bpID)
				flatMat = append(flatMat, m.MaterialTypeID)
				flatQty = append(flatQty, m.BaseQuantity)
			}
		}

		for start := 0; start < len(touchedBPs); start += batchSize {
			end := start + batchSize
			if end > len(touchedBPs) {
				end = len(touchedBPs)
			}
			if _, err := tx.Exec(ctx, `DELETE FROM sde_blueprint_materials WHERE blueprint_type_id = ANY($1::int[])`, touchedBPs[start:end]); err != nil {
				return fmt.Errorf("clearing materials batch: %w", err)
			}
		}

		for start := 0; start < len(flatBP); start += batchSize {
			end := start + batchSize
			if end > len(flatBP) {
				end = len(flatBP)
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO sde_blueprint_materials (blueprint_type_id, material_type_id, base_quantity)
				SELECT * FROM UNNEST($1::int[], $2::int[], $3::bigint[])
				ON CONFLICT (blueprint_type_id, material_type_id) DO UPDATE SET
					base_quantity = EXCLUDED.base_quantity
			`, flatBP[start:end], flatMat[start:end], flatQty[start:end]); err != nil {
				return fmt.Errorf("bulk upserting sde_blueprint_materials batch %d-%d: %w", start, end, err)
			}
		}
	}

	return tx.Commit(ctx)
}

// SearchBlueprintsPaged returns a paginated slice of blueprints filtered by query, category, and tech level,
// along with total count and all available categories for the Blueprint Catalog Modal Popup.
func (r *SDERepo) SearchBlueprintsPaged(ctx context.Context, query, category string, techLevel int, page, pageSize int) (*domain.BlueprintPageResult, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 60 {
		pageSize = 24
	}
	query = strings.TrimSpace(query)
	category = strings.TrimSpace(category)
	if strings.EqualFold(category, "ALL") {
		category = ""
	}

	var total int
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)::int
		FROM sde_blueprints
		WHERE ($1 = '' OR LOWER(name) LIKE '%' || LOWER($1) || '%' OR LOWER(product_name) LIKE '%' || LOWER($1) || '%')
		  AND ($2 = '' OR LOWER(category) = LOWER($2))
		  AND ($3 <= 0 OR tech_level = $3)
	`, query, category, techLevel).Scan(&total); err != nil {
		return nil, fmt.Errorf("counting sde_blueprints: %w", err)
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))
	if totalPages < 1 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}
	offset := (page - 1) * pageSize

	rows, err := r.pool.Query(ctx, `
		SELECT blueprint_type_id, name, product_type_id, product_name, product_qty_per_run, base_time_seconds, tech_level, category, activity_type
		FROM sde_blueprints
		WHERE ($1 = '' OR LOWER(name) LIKE '%' || LOWER($1) || '%' OR LOWER(product_name) LIKE '%' || LOWER($1) || '%')
		  AND ($2 = '' OR LOWER(category) = LOWER($2))
		  AND ($3 <= 0 OR tech_level = $3)
		ORDER BY tech_level ASC, category ASC, product_name ASC
		LIMIT $4 OFFSET $5
	`, query, category, techLevel, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("searching paged sde_blueprints: %w", err)
	}
	defer rows.Close()

	items := make([]domain.SDEBlueprint, 0, pageSize)
	for rows.Next() {
		var bp domain.SDEBlueprint
		if err := rows.Scan(
			&bp.BlueprintTypeID, &bp.Name, &bp.ProductTypeID, &bp.ProductName,
			&bp.ProductQtyPerRun, &bp.BaseTimeSeconds, &bp.TechLevel, &bp.Category, &bp.ActivityType,
		); err != nil {
			return nil, fmt.Errorf("scanning paged sde_blueprint: %w", err)
		}
		items = append(items, bp)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	catRows, err := r.pool.Query(ctx, `SELECT DISTINCT category FROM sde_blueprints WHERE category <> '' ORDER BY category ASC`)
	categories := make([]string, 0, 16)
	if err == nil {
		defer catRows.Close()
		for catRows.Next() {
			var c string
			if err := catRows.Scan(&c); err == nil {
				categories = append(categories, c)
			}
		}
	}

	return &domain.BlueprintPageResult{
		Items:      items,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
		Categories: categories,
	}, nil
}

// GetSDEMetadata returns the currently installed SDE metadata, falling back to live table counts for the starter set.
func (r *SDERepo) GetSDEMetadata(ctx context.Context) (*domain.SDEMetadata, error) {
	var m domain.SDEMetadata
	err := r.pool.QueryRow(ctx, `
		SELECT build_number, release_date, source, types_count, blueprints_count, materials_count, inventions_count, updated_at
		FROM sde_metadata
		WHERE id = 1
	`).Scan(&m.BuildNumber, &m.ReleaseDate, &m.Source, &m.TypesCount, &m.BlueprintsCount, &m.MaterialsCount, &m.InventionsCount, &m.UpdatedAt)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("reading sde_metadata: %w", err)
		}
		m.Source = "starter"
		m.UpdatedAt = time.Now()
	}

	// Always reflect live blueprint, type, and invention counts accurately
	_ = r.pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM sde_types`).Scan(&m.TypesCount)
	_ = r.pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM sde_blueprints`).Scan(&m.BlueprintsCount)
	_ = r.pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM sde_blueprint_materials`).Scan(&m.MaterialsCount)
	_ = r.pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM sde_inventions`).Scan(&m.InventionsCount)
	return &m, nil
}

// SaveSDEMetadata persists the installed CCP SDE build number and record counts.
func (r *SDERepo) SaveSDEMetadata(ctx context.Context, meta *domain.SDEMetadata) error {
	if meta == nil {
		return nil
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO sde_metadata (id, build_number, release_date, source, types_count, blueprints_count, materials_count, inventions_count, updated_at)
		VALUES (1, $1, $2, $3, $4, $5, $6, $7, NOW())
		ON CONFLICT (id) DO UPDATE SET
			build_number = EXCLUDED.build_number,
			release_date = EXCLUDED.release_date,
			source = EXCLUDED.source,
			types_count = EXCLUDED.types_count,
			blueprints_count = EXCLUDED.blueprints_count,
			materials_count = EXCLUDED.materials_count,
			inventions_count = EXCLUDED.inventions_count,
			updated_at = NOW()
	`, meta.BuildNumber, meta.ReleaseDate, meta.Source, meta.TypesCount, meta.BlueprintsCount, meta.MaterialsCount, meta.InventionsCount)
	return err
}

// UpsertInventionData bulk-upserts official CCP SDE T1 -> T2 invention pathways and datacore requirements.
func (r *SDERepo) UpsertInventionData(ctx context.Context, inventions []domain.SDEInvention, materials map[int32][]domain.SDEInventionMaterial) error {
	if len(inventions) == 0 {
		return nil
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("starting invention upsert tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const batchSize = 500
	for start := 0; start < len(inventions); start += batchSize {
		end := start + batchSize
		if end > len(inventions) {
			end = len(inventions)
		}
		chunk := inventions[start:end]
		t1IDs := make([]int32, len(chunk))
		t2IDs := make([]int32, len(chunk))
		prodIDs := make([]int32, len(chunk))
		prodNames := make([]string, len(chunk))
		probs := make([]float64, len(chunk))
		runs := make([]int32, len(chunk))
		times := make([]int32, len(chunk))
		cats := make([]string, len(chunk))
		for i, inv := range chunk {
			t1IDs[i] = inv.T1BlueprintTypeID
			t2IDs[i] = inv.T2BlueprintTypeID
			prodIDs[i] = inv.T2ProductTypeID
			prodNames[i] = inv.T2ProductName
			probs[i] = inv.BaseProbability
			runs[i] = int32(inv.BaseRuns)
			times[i] = int32(inv.BaseTimeSeconds)
			cats[i] = inv.Category
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO sde_inventions (t1_blueprint_type_id, t2_blueprint_type_id, t2_product_type_id, t2_product_name, base_probability, base_runs, base_time_seconds, category)
			SELECT * FROM UNNEST($1::int[], $2::int[], $3::int[], $4::varchar[], $5::double precision[], $6::int[], $7::int[], $8::varchar[])
			ON CONFLICT (t1_blueprint_type_id, t2_blueprint_type_id) DO UPDATE SET
				t2_product_type_id = EXCLUDED.t2_product_type_id,
				t2_product_name = EXCLUDED.t2_product_name,
				base_probability = EXCLUDED.base_probability,
				base_runs = EXCLUDED.base_runs,
				base_time_seconds = EXCLUDED.base_time_seconds,
				category = EXCLUDED.category
		`, t1IDs, t2IDs, prodIDs, prodNames, probs, runs, times, cats); err != nil {
			return fmt.Errorf("bulk upserting sde_inventions: %w", err)
		}
	}

	if len(materials) > 0 {
		touched := make([]int32, 0, len(materials))
		flatT1 := make([]int32, 0, len(materials)*2)
		flatMat := make([]int32, 0, len(materials)*2)
		flatQty := make([]int32, 0, len(materials)*2)
		for t1ID, mats := range materials {
			touched = append(touched, t1ID)
			for _, m := range mats {
				if m.MaterialTypeID <= 0 || m.Quantity <= 0 {
					continue
				}
				flatT1 = append(flatT1, t1ID)
				flatMat = append(flatMat, m.MaterialTypeID)
				flatQty = append(flatQty, int32(m.Quantity))
			}
		}
		for start := 0; start < len(touched); start += batchSize {
			end := start + batchSize
			if end > len(touched) {
				end = len(touched)
			}
			if _, err := tx.Exec(ctx, `DELETE FROM sde_invention_materials WHERE t1_blueprint_type_id = ANY($1::int[])`, touched[start:end]); err != nil {
				return fmt.Errorf("clearing sde_invention_materials: %w", err)
			}
		}
		for start := 0; start < len(flatT1); start += batchSize {
			end := start + batchSize
			if end > len(flatT1) {
				end = len(flatT1)
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO sde_invention_materials (t1_blueprint_type_id, material_type_id, quantity)
				SELECT * FROM UNNEST($1::int[], $2::int[], $3::int[])
				ON CONFLICT (t1_blueprint_type_id, material_type_id) DO UPDATE SET
					quantity = EXCLUDED.quantity
			`, flatT1[start:end], flatMat[start:end], flatQty[start:end]); err != nil {
				return fmt.Errorf("bulk upserting sde_invention_materials: %w", err)
			}
		}
	}

	return tx.Commit(ctx)
}

// SearchInventions searches official CCP SDE T1 -> T2 invention pathways by T2 product name.
func (r *SDERepo) SearchInventions(ctx context.Context, query string, limit int) ([]domain.SDEInvention, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	query = strings.TrimSpace(query)
	rows, err := r.pool.Query(ctx, `
		SELECT t1_blueprint_type_id, t2_blueprint_type_id, t2_product_type_id, t2_product_name, base_probability, base_runs, base_time_seconds, category
		FROM sde_inventions
		WHERE ($1 = '' OR LOWER(t2_product_name) LIKE '%' || LOWER($1) || '%')
		ORDER BY category ASC, t2_product_name ASC
		LIMIT $2
	`, query, limit)
	if err != nil {
		return nil, fmt.Errorf("searching sde_inventions: %w", err)
	}
	defer rows.Close()

	out := make([]domain.SDEInvention, 0)
	for rows.Next() {
		var inv domain.SDEInvention
		if err := rows.Scan(
			&inv.T1BlueprintTypeID, &inv.T2BlueprintTypeID, &inv.T2ProductTypeID, &inv.T2ProductName,
			&inv.BaseProbability, &inv.BaseRuns, &inv.BaseTimeSeconds, &inv.Category,
		); err != nil {
			return nil, fmt.Errorf("scanning sde_invention: %w", err)
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

// GetInventionByT2BlueprintID looks up the T1 invention pathway and datacore materials for a T2 BlueprintTypeID.
func (r *SDERepo) GetInventionByT2BlueprintID(ctx context.Context, t2BlueprintTypeID int32) (*domain.SDEInvention, []domain.SDEInventionMaterial, error) {
	var inv domain.SDEInvention
	err := r.pool.QueryRow(ctx, `
		SELECT t1_blueprint_type_id, t2_blueprint_type_id, t2_product_type_id, t2_product_name, base_probability, base_runs, base_time_seconds, category
		FROM sde_inventions
		WHERE t2_blueprint_type_id = $1
		LIMIT 1
	`, t2BlueprintTypeID).Scan(
		&inv.T1BlueprintTypeID, &inv.T2BlueprintTypeID, &inv.T2ProductTypeID, &inv.T2ProductName,
		&inv.BaseProbability, &inv.BaseRuns, &inv.BaseTimeSeconds, &inv.Category,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("getting sde_invention for t2 bp %d: %w", t2BlueprintTypeID, err)
	}

	matRows, err := r.pool.Query(ctx, `
		SELECT m.t1_blueprint_type_id, m.material_type_id, t.name, m.quantity
		FROM sde_invention_materials m
		JOIN sde_types t ON t.type_id = m.material_type_id
		WHERE m.t1_blueprint_type_id = $1
		ORDER BY t.name ASC
	`, inv.T1BlueprintTypeID)
	if err != nil {
		return &inv, nil, fmt.Errorf("getting sde_invention_materials: %w", err)
	}
	defer matRows.Close()

	mats := make([]domain.SDEInventionMaterial, 0, 4)
	for matRows.Next() {
		var m domain.SDEInventionMaterial
		if err := matRows.Scan(&m.T1BlueprintTypeID, &m.MaterialTypeID, &m.MaterialName, &m.Quantity); err != nil {
			return &inv, nil, err
		}
		mats = append(mats, m)
	}
	return &inv, mats, matRows.Err()
}

// ListReactionBlueprints returns reaction formulas (activity_type = 'reaction') stored in sde_blueprints.
func (r *SDERepo) ListReactionBlueprints(ctx context.Context, query string, limit int) ([]domain.SDEBlueprint, error) {
	if limit <= 0 || limit > 200 {
		limit = 60
	}
	query = strings.TrimSpace(query)
	rows, err := r.pool.Query(ctx, `
		SELECT blueprint_type_id, name, product_type_id, product_name, product_qty_per_run, base_time_seconds, tech_level, category, activity_type
		FROM sde_blueprints
		WHERE activity_type = 'reaction'
		  AND ($1 = '' OR LOWER(name) LIKE '%' || LOWER($1) || '%' OR LOWER(product_name) LIKE '%' || LOWER($1) || '%')
		ORDER BY product_name ASC
		LIMIT $2
	`, query, limit)
	if err != nil {
		return nil, fmt.Errorf("listing reaction blueprints: %w", err)
	}
	defer rows.Close()

	out := make([]domain.SDEBlueprint, 0)
	for rows.Next() {
		var bp domain.SDEBlueprint
		if err := rows.Scan(
			&bp.BlueprintTypeID, &bp.Name, &bp.ProductTypeID, &bp.ProductName,
			&bp.ProductQtyPerRun, &bp.BaseTimeSeconds, &bp.TechLevel, &bp.Category, &bp.ActivityType,
		); err != nil {
			return nil, err
		}
		out = append(out, bp)
	}
	return out, rows.Err()
}

// GetTypeTraits returns the parsed CCP SDE Hull Bonuses (Traits) for a given EVE type_id, or nil if none exist.
func (r *SDERepo) GetTypeTraits(ctx context.Context, typeID int32) (*domain.ItemTraits, error) {
	if typeID <= 0 {
		return nil, nil
	}
	var raw string
	err := r.pool.QueryRow(ctx, `SELECT traits_json FROM sde_types WHERE type_id = $1`, typeID).Scan(&raw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting traits for type %d: %w", typeID, err)
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var tr domain.ItemTraits
	if err := json.Unmarshal([]byte(raw), &tr); err != nil {
		return nil, nil
	}
	if !tr.HasAny() {
		return nil, nil
	}
	return &tr, nil
}
