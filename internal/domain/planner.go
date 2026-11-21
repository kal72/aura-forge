package domain

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// PlanStatus defines the operational lifecycle state of a weekly plan.
type PlanStatus string

const (
	PlanStatusDraft     PlanStatus = "DRAFT"
	PlanStatusActive    PlanStatus = "ACTIVE"
	PlanStatusCompleted PlanStatus = "COMPLETED"
	PlanStatusArchived  PlanStatus = "ARCHIVED"
)

const (
	HoursPerWeek             = 168.0
	DefaultReinvestRate      = 0.85 // Reinvest 85% of weekly profit, keep 15% buffer
	DefaultProjectionWeeks   = 4
	Tier2CapitalThresholdISK = 150_000_000.0
	Tier3CapitalThresholdISK = 500_000_000.0
	Tier4CapitalThresholdISK = 1_500_000_000.0
)

var (
	ErrInvalidSimulationInput = errors.New("starting capital must be positive and reinvest rate between 0 and 1")
)

// DaySchedule outlines operational actions scheduled for a specific day of the week.
type DaySchedule struct {
	DayNumber        int      `json:"day_number"` // 1 through 7
	Title            string   `json:"title"`      // e.g. "Day 1: Sourcing & Buy Orders"
	Focus            string   `json:"focus"`
	Tasks            []string `json:"tasks"`
	BatchesScheduled []int64  `json:"batches_scheduled"` // Associated Batch IDs
}

// PlanItem represents an item target in the weekly production plan.
type PlanItem struct {
	TypeID              int32   `json:"type_id"`
	Name                string  `json:"name"`
	BatchRuns           int     `json:"batch_runs"`
	OutputPerBatch      int     `json:"output_per_batch"`
	TotalUnits          int     `json:"total_units"`
	UnitSellPrice       float64 `json:"unit_sell_price"`
	TotalMaterialCost   float64 `json:"total_material_cost"`
	TotalCost           float64 `json:"total_cost"`
	TotalGrossRevenue   float64 `json:"total_gross_revenue"`
	NetProfit           float64 `json:"net_profit"`
	ProductionTimeHours float64 `json:"production_time_hours"`
	RequiresScienceSlot bool    `json:"requires_science_slot"`
}

// WeeklyPlan models the entire 7-day manufacturing and trade campaign.
type WeeklyPlan struct {
	ID                      int64         `json:"id"`
	Title                   string        `json:"title"`
	WeekNumber              int           `json:"week_number"`
	StartingCash            float64       `json:"starting_cash"`
	Status                  PlanStatus    `json:"status"`
	ManufacturingSlotsTotal int           `json:"manufacturing_slots_total"`
	ManufacturingSlotsUsed  int           `json:"manufacturing_slots_used"`
	ScienceSlotsTotal       int           `json:"science_slots_total"`
	ScienceSlotsUsed        int           `json:"science_slots_used"`
	Days                    []DaySchedule `json:"days"`
	Items                   []PlanItem    `json:"items"`
	TotalEstimatedCost      float64       `json:"total_estimated_cost"`
	TotalEstimatedRevenue   float64       `json:"total_estimated_revenue"`
	TotalEstimatedProfit    float64       `json:"total_estimated_profit"`
	ProjectedROI            float64       `json:"projected_roi"`
	CreatedAt               time.Time     `json:"created_at"`
	UpdatedAt               time.Time     `json:"updated_at"`
}

// SlotCapacityReport evaluates whether a WeeklyPlan fits within character slot limits.
type SlotCapacityReport struct {
	ManufacturingSlotsTotal    int     `json:"manufacturing_slots_total"`
	ManufacturingSlotsRequired int     `json:"manufacturing_slots_required"`
	ScienceSlotsTotal          int     `json:"science_slots_total"`
	ScienceSlotsRequired       int     `json:"science_slots_required"`
	TotalProductionHours       float64 `json:"total_production_hours"`
	WeeklySlotCapacityHours    float64 `json:"weekly_slot_capacity_hours"`
	UtilizationPercent         float64 `json:"utilization_percent"`
	FitsSlotCapacity           bool    `json:"fits_slot_capacity"`
	Warning                    string  `json:"warning"`
}

// MaterialRequirement represents a raw material requirement prior to consolidation.
type MaterialRequirement struct {
	TypeID       int32   `json:"type_id"`
	TypeName     string  `json:"type_name"`
	Quantity     int64   `json:"quantity"`
	UnitPrice    float64 `json:"unit_price"`
	UnitVolumeM3 float64 `json:"unit_volume_m3"`
}

// ShoppingListItem represents aggregated bill of materials for weekly production.
type ShoppingListItem struct {
	TypeID             int32   `json:"type_id"`
	TypeName           string  `json:"type_name"`
	RequiredQuantity   int64   `json:"required_quantity"`
	HangarQuantity     int64   `json:"hangar_quantity"`
	ToBuyQuantity      int64   `json:"to_buy_quantity"`
	EstimatedUnitPrice float64 `json:"estimated_unit_price"`
	TotalCost          float64 `json:"total_cost"`
	VolumeM3           float64 `json:"volume_m3"`
}

// ConsolidatedShoppingList holds the deduplicated BOM, hangar deductions, and EVE Multibuy export text.
type ConsolidatedShoppingList struct {
	Items              []ShoppingListItem `json:"items"`
	TotalEstimatedCost float64            `json:"total_estimated_cost"`
	TotalVolumeM3      float64            `json:"total_volume_m3"`
	TotalItemsDeducted int64              `json:"total_items_deducted"`
	MultibuyText       string             `json:"multibuy_text"`
}

// GrowthProjection simulates 4-week compounding capital growth.
type GrowthProjection struct {
	Week              int     `json:"week"`
	StartingCapital   float64 `json:"starting_capital"`
	WeeklyProfit      float64 `json:"weekly_profit"`
	ReinvestedProfit  float64 `json:"reinvested_profit"`
	EndingCapital     float64 `json:"ending_capital"`
	RecommendedTier   string  `json:"recommended_tier"`
	StrategicGuidance string  `json:"strategic_guidance"`
}

// BuildWeeklyPlan constructs a validated WeeklyPlan with Day 1-7 schedule and financial totals.
func BuildWeeklyPlan(
	title string,
	weekNumber int,
	startingCash float64,
	mfgSlotsTotal int,
	sciSlotsTotal int,
	items []PlanItem,
) WeeklyPlan {
	if weekNumber <= 0 {
		weekNumber = 1
	}
	if mfgSlotsTotal <= 0 {
		mfgSlotsTotal = 1
	}
	if sciSlotsTotal < 0 {
		sciSlotsTotal = 0
	}
	if strings.TrimSpace(title) == "" {
		title = fmt.Sprintf("Weekly Production Plan #%d", weekNumber)
	}

	var totalCost, totalRev, totalProfit float64
	mfgUsed := 0
	sciUsed := 0

	for i := range items {
		if items[i].BatchRuns <= 0 {
			items[i].BatchRuns = 1
		}
		if items[i].OutputPerBatch <= 0 {
			items[i].OutputPerBatch = 1
		}
		items[i].TotalUnits = items[i].BatchRuns * items[i].OutputPerBatch
		totalCost += items[i].TotalCost
		totalRev += items[i].TotalGrossRevenue
		totalProfit += items[i].NetProfit
		mfgUsed += items[i].BatchRuns
		if items[i].RequiresScienceSlot {
			sciUsed++
		}
	}

	var roi float64
	if totalCost > 0 {
		roi = RoundPercent((totalProfit / totalCost) * 100.0)
	}

	now := time.Now()
	return WeeklyPlan{
		Title:                   title,
		WeekNumber:              weekNumber,
		StartingCash:            RoundISK(startingCash),
		Status:                  PlanStatusDraft,
		ManufacturingSlotsTotal: mfgSlotsTotal,
		ManufacturingSlotsUsed:  mfgUsed,
		ScienceSlotsTotal:       sciSlotsTotal,
		ScienceSlotsUsed:        sciUsed,
		Days:                    GenerateWeeklySchedule(items, mfgSlotsTotal),
		Items:                   items,
		TotalEstimatedCost:      RoundISK(totalCost),
		TotalEstimatedRevenue:   RoundISK(totalRev),
		TotalEstimatedProfit:    RoundISK(totalProfit),
		ProjectedROI:            roi,
		CreatedAt:               now,
		UpdatedAt:               now,
	}
}

// ValidateSlotCapacity checks if the planned manufacturing and science jobs fit within weekly capacity.
func ValidateSlotCapacity(plan WeeklyPlan) SlotCapacityReport {
	mfgTotal := plan.ManufacturingSlotsTotal
	if mfgTotal <= 0 {
		mfgTotal = 1
	}

	var totalHours float64
	mfgRequired := 0
	sciRequired := 0

	for _, item := range plan.Items {
		runs := item.BatchRuns
		if runs <= 0 {
			runs = 1
		}
		mfgRequired += runs
		totalHours += item.ProductionTimeHours * float64(runs)
		if item.RequiresScienceSlot {
			sciRequired++
		}
	}

	weeklyCapHours := float64(mfgTotal) * HoursPerWeek
	utilization := 0.0
	if weeklyCapHours > 0 {
		utilization = RoundPercent((totalHours / weeklyCapHours) * 100.0)
	}

	fits := true
	var warnings []string

	if totalHours > weeklyCapHours {
		fits = false
		warnings = append(warnings, fmt.Sprintf("Total production time (%.1f hrs) exceeds weekly capacity for %d slot(s) (%.1f hrs).", totalHours, mfgTotal, weeklyCapHours))
	}
	if sciRequired > plan.ScienceSlotsTotal {
		fits = false
		warnings = append(warnings, fmt.Sprintf("Science/Invention slots required (%d) exceed capacity (%d).", sciRequired, plan.ScienceSlotsTotal))
	}
	if mfgRequired > mfgTotal*3 && totalHours == 0 {
		// Heuristic when production hours aren't specified: warn if >3 waves per week
		warnings = append(warnings, fmt.Sprintf("High batch count (%d) for %d manufacturing slot(s); split execution across multiple waves.", mfgRequired, mfgTotal))
	}

	return SlotCapacityReport{
		ManufacturingSlotsTotal:    mfgTotal,
		ManufacturingSlotsRequired: mfgRequired,
		ScienceSlotsTotal:          plan.ScienceSlotsTotal,
		ScienceSlotsRequired:       sciRequired,
		TotalProductionHours:       RoundPercent(totalHours),
		WeeklySlotCapacityHours:    RoundPercent(weeklyCapHours),
		UtilizationPercent:         utilization,
		FitsSlotCapacity:           fits,
		Warning:                    strings.Join(warnings, " "),
	}
}

// GenerateWeeklySchedule produces an actionable Day 1 - Day 7 execution blueprint.
func GenerateWeeklySchedule(items []PlanItem, totalMfgSlots int) []DaySchedule {
	if totalMfgSlots <= 0 {
		totalMfgSlots = 1
	}

	var itemSummaries []string
	for _, it := range items {
		itemSummaries = append(itemSummaries, fmt.Sprintf("%s (%dx batch)", it.Name, it.BatchRuns))
	}

	wave1Target := "Fast Cashflow & Mid-Tier items"
	if len(itemSummaries) > 0 {
		limit := len(itemSummaries)
		if limit > 3 {
			limit = 3
		}
		wave1Target = strings.Join(itemSummaries[:limit], ", ")
	}

	return []DaySchedule{
		{
			DayNumber: 1,
			Title:     "Day 1: Sourcing & Buy Orders",
			Focus:     "Material Procurement",
			Tasks: []string{
				"Copy the Consolidated Shopping List below into EVE Online Multibuy.",
				"Place Buy Orders at your Primary Trade Hub for high-value minerals/components to save 5%–10%.",
				"Haul raw materials to a low System Cost Index (SCI) manufacturing system.",
			},
			BatchesScheduled: []int64{},
		},
		{
			DayNumber: 2,
			Title:     "Day 2: Wave 1 Production",
			Focus:     "Initial Slot Execution",
			Tasks: []string{
				fmt.Sprintf("Fill up to %d Manufacturing Slot(s) with Wave 1 targets: %s.", totalMfgSlots, wave1Target),
				"Verify job installation fees (SCI + Facility Tax) match your calculator estimate.",
			},
			BatchesScheduled: []int64{},
		},
		{
			DayNumber: 3,
			Title:     "Day 3: Initial Listing & Hub Check",
			Focus:     "Distribution & Sales",
			Tasks: []string{
				"Deliver Wave 1 completed jobs and haul finished goods to your Primary Trade Hub.",
				"List Sell Orders competitively (1 tick below lowest sell) and mark batches LISTED in Tracker.",
			},
			BatchesScheduled: []int64{},
		},
		{
			DayNumber: 4,
			Title:     "Day 4: Wave 2 Re-Batching",
			Focus:     "Cashflow Turnover",
			Tasks: []string{
				"Reinvest incoming ISK from Fast Cashflow sales into fresh raw materials.",
				"Start Wave 2 production batches in any newly freed manufacturing slots.",
			},
			BatchesScheduled: []int64{},
		},
		{
			DayNumber: 5,
			Title:     "Day 5: Undercut Radar & Safe Relist",
			Focus:     "Margin Protection",
			Tasks: []string{
				"Run Check Live Undercuts in Tracker to spot undercut Sell Orders.",
				"Check Safe Relist Floor before modifying prices so broker & relist fees never erase profit.",
			},
			BatchesScheduled: []int64{},
		},
		{
			DayNumber: 6,
			Title:     "Day 6: Weekend Traffic Peak",
			Focus:     "Maximize Sales Volume",
			Tasks: []string{
				"Ensure all finished inventory is listed at your Trade Hub ahead of peak weekend player activity.",
				"Queue longer-duration batches (High ROI / Ships / T2) to keep slots productive while offline.",
			},
			BatchesScheduled: []int64{},
		},
		{
			DayNumber: 7,
			Title:     "Day 7: Profit Harvest & Weekly Audit",
			Focus:     "Review & Scale Up",
			Tasks: []string{
				"Mark sold batches as SOLD in Tracker and review realized weekly profit.",
				"Compare capital growth against the Week 1–4 projection and prepare next week's allocation.",
			},
			BatchesScheduled: []int64{},
		},
	}
}

// BuildConsolidatedShoppingList aggregates materials, deducts hangar inventory, and builds EVE Multibuy text.
func BuildConsolidatedShoppingList(requirements []MaterialRequirement, hangarStock map[int32]int64) ConsolidatedShoppingList {
	type aggEntry struct {
		typeID       int32
		typeName     string
		requiredQty  int64
		unitPrice    float64
		unitVolumeM3 float64
	}

	byKey := make(map[string]*aggEntry)
	for _, req := range requirements {
		if req.Quantity <= 0 || strings.TrimSpace(req.TypeName) == "" {
			continue
		}
		key := fmt.Sprintf("%d:%s", req.TypeID, strings.ToLower(strings.TrimSpace(req.TypeName)))
		if req.TypeID > 0 {
			key = fmt.Sprintf("id:%d", req.TypeID)
		}
		if existing, ok := byKey[key]; ok {
			existing.requiredQty += req.Quantity
			if existing.unitPrice == 0 && req.UnitPrice > 0 {
				existing.unitPrice = req.UnitPrice
			}
			if existing.unitVolumeM3 == 0 && req.UnitVolumeM3 > 0 {
				existing.unitVolumeM3 = req.UnitVolumeM3
			}
		} else {
			byKey[key] = &aggEntry{
				typeID:       req.TypeID,
				typeName:     strings.TrimSpace(req.TypeName),
				requiredQty:  req.Quantity,
				unitPrice:    req.UnitPrice,
				unitVolumeM3: req.UnitVolumeM3,
			}
		}
	}

	items := make([]ShoppingListItem, 0, len(byKey))
	var totalCost, totalVol float64
	var totalDeducted int64

	for _, entry := range byKey {
		var hangarQty int64
		if hangarStock != nil && entry.typeID > 0 {
			hangarQty = hangarStock[entry.typeID]
			if hangarQty < 0 {
				hangarQty = 0
			}
		}

		deducted := hangarQty
		if deducted > entry.requiredQty {
			deducted = entry.requiredQty
		}
		toBuy := entry.requiredQty - deducted

		cost := RoundISK(float64(toBuy) * entry.unitPrice)
		vol := RoundISK(float64(toBuy) * entry.unitVolumeM3)

		totalCost += cost
		totalVol += vol
		totalDeducted += deducted

		items = append(items, ShoppingListItem{
			TypeID:             entry.typeID,
			TypeName:           entry.typeName,
			RequiredQuantity:   entry.requiredQty,
			HangarQuantity:     hangarQty,
			ToBuyQuantity:      toBuy,
			EstimatedUnitPrice: RoundISK(entry.unitPrice),
			TotalCost:          cost,
			VolumeM3:           vol,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].TotalCost == items[j].TotalCost {
			return items[i].TypeName < items[j].TypeName
		}
		return items[i].TotalCost > items[j].TotalCost
	})

	return ConsolidatedShoppingList{
		Items:              items,
		TotalEstimatedCost: RoundISK(totalCost),
		TotalVolumeM3:      RoundISK(totalVol),
		TotalItemsDeducted: totalDeducted,
		MultibuyText:       FormatEVEMultibuy(items),
	}
}

// FormatEVEMultibuy formats shopping list items with ToBuyQuantity > 0 into EVE Online Multibuy clipboard syntax.
func FormatEVEMultibuy(items []ShoppingListItem) string {
	var lines []string
	for _, it := range items {
		if it.ToBuyQuantity <= 0 || strings.TrimSpace(it.TypeName) == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s\t%d", it.TypeName, it.ToBuyQuantity))
	}
	return strings.Join(lines, "\n")
}

// SimulateFourWeekGrowth models compound capital expansion from Week 1 to Week 4.
func SimulateFourWeekGrowth(startingCapital, weeklyROIPercent, reinvestRate float64) ([]GrowthProjection, error) {
	if startingCapital <= 0 || reinvestRate < 0 || reinvestRate > 1.0 {
		return nil, ErrInvalidSimulationInput
	}
	if weeklyROIPercent < 0 {
		weeklyROIPercent = 0
	}

	projections := make([]GrowthProjection, 0, DefaultProjectionWeeks)
	currentCap := RoundISK(startingCapital)

	for week := 1; week <= DefaultProjectionWeeks; week++ {
		// Deployable capital assumes 85% deployed (15% cash buffer)
		deployable := currentCap * 0.85
		weeklyProfit := RoundISK(deployable * (weeklyROIPercent / 100.0))
		reinvested := RoundISK(weeklyProfit * reinvestRate)
		endingCap := RoundISK(currentCap + reinvested)

		tier, guidance := recommendCapitalTier(endingCap)
		projections = append(projections, GrowthProjection{
			Week:              week,
			StartingCapital:   currentCap,
			WeeklyProfit:      weeklyProfit,
			ReinvestedProfit:  reinvested,
			EndingCapital:     endingCap,
			RecommendedTier:   tier,
			StrategicGuidance: guidance,
		})

		currentCap = endingCap
	}

	return projections, nil
}

func recommendCapitalTier(capital float64) (string, string) {
	switch {
	case capital >= Tier4CapitalThresholdISK:
		return "Tier 4 — Industrial Scale (T2 Hulls & Regional)", "Capital ready to scale into T2 Cruisers/Battleships, moon reactions, or multi-hub arbitrage."
	case capital >= Tier3CapitalThresholdISK:
		return "Tier 3 — T2 Modules & Invention", "Sufficient working capital to start T2 Invention (Modules/Drones/Ammo) for thicker margins."
	case capital >= Tier2CapitalThresholdISK:
		return "Tier 2 — T1 Frigates, Destroyers & Rigs", "Step up to high-volume T1 PvP/Faction Warfare hulls and popular rigs."
	default:
		return "Tier 1 — Fast Cashflow (Ammo, Drones & Basic Modules)", "Focus on rapid ISK turnover and keep a 15% cash buffer for taxes and broker fees."
	}
}

// ConvertPlanToBatches converts a WeeklyPlan's PlanItems into Draft BatchRecords ready for the Tracker.
func ConvertPlanToBatches(plan WeeklyPlan, settings UserSettings) []BatchRecord {
	batches := make([]BatchRecord, 0, len(plan.Items))
	now := time.Now()
	planID := plan.ID

	for _, item := range plan.Items {
		units := item.TotalUnits
		if units <= 0 {
			units = item.BatchRuns * item.OutputPerBatch
		}
		if units <= 0 {
			units = 1
		}

		safeRelist := CalculateSafeRelistPrice(
			item.TotalCost,
			units,
			settings.SalesTaxPercent,
			settings.RelistFeePercent,
		)

		remainingMargin := RoundISK((item.UnitSellPrice - safeRelist) * float64(units))
		if remainingMargin < 0 {
			remainingMargin = 0
		}

		var roi float64
		if item.TotalCost > 0 {
			roi = RoundPercent((item.NetProfit / item.TotalCost) * 100.0)
		}

		var planIDPtr *int64
		if planID > 0 {
			idCopy := planID
			planIDPtr = &idCopy
		}

		batches = append(batches, BatchRecord{
			PlanID:               planIDPtr,
			TypeID:               item.TypeID,
			ItemName:             item.Name,
			Runs:                 item.BatchRuns,
			UnitsOutput:          units,
			TotalMaterialCost:    RoundISK(item.TotalMaterialCost),
			ActualProductionCost: RoundISK(item.TotalCost),
			ActualSellPrice:      RoundISK(item.UnitSellPrice),
			TotalRevenue:         RoundISK(item.TotalGrossRevenue),
			NetProfit:            RoundISK(item.NetProfit),
			ROI:                  roi,
			Status:               BatchStatusDraft,
			CurrentMarketPrice:   RoundISK(item.UnitSellPrice),
			IsUndercut:           false,
			SafeRelistPrice:      safeRelist,
			RemainingSafeMargin:  remainingMargin,
			CreatedAt:            now,
			UpdatedAt:            now,
		})
	}

	return batches
}
