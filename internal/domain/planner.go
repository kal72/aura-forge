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
		title = fmt.Sprintf("Rencana Produksi Minggu #%d", weekNumber)
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
		warnings = append(warnings, fmt.Sprintf("Total waktu produksi (%.1f jam) melebihi kapasitas mingguan %d slot (%.1f jam).", totalHours, mfgTotal, weeklyCapHours))
	}
	if sciRequired > plan.ScienceSlotsTotal {
		fits = false
		warnings = append(warnings, fmt.Sprintf("Kebutuhan Science/Invention slot (%d) melebihi kapasitas (%d).", sciRequired, plan.ScienceSlotsTotal))
	}
	if mfgRequired > mfgTotal*3 && totalHours == 0 {
		// Heuristic when production hours aren't specified: warn if >3 waves per week
		warnings = append(warnings, fmt.Sprintf("Jumlah batch (%d) tinggi untuk %d slot manufaktur; pastikan jadwal dibagi ke beberapa gelombang.", mfgRequired, mfgTotal))
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

	wave1Target := "Item Fast Cashflow & Mid-Tier"
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
			Focus:     "Pengadaan Bahan Baku",
			Tasks: []string{
				"Copy Consolidated Shopping List ke format EVE Multibuy.",
				"Pasang Buy Order di Jita 4-4 untuk mineral/komponen bernilai besar agar hemat 5%–10%.",
				"Angkut bahan baku ke sistem pabrik dengan System Cost Index (SCI) rendah.",
			},
			BatchesScheduled: []int64{},
		},
		{
			DayNumber: 2,
			Title:     "Day 2: Produksi Gelombang 1",
			Focus:     "Eksekusi Slot Awal",
			Tasks: []string{
				fmt.Sprintf("Isi hingga %d Manufacturing Slot dengan target awal: %s.", totalMfgSlots, wave1Target),
				"Verifikasi biaya instalasi job (SCI + Facility Tax) sesuai kalkulator.",
			},
			BatchesScheduled: []int64{},
		},
		{
			DayNumber: 3,
			Title:     "Day 3: Listing Awal & Cek Jita",
			Focus:     "Distribusi & Penjualan",
			Tasks: []string{
				"Deliver job gelombang 1 dan angkut hasil produksi ke Jita 4-4.",
				"Pasang Sell Order dengan harga kompetitif (1 tick di bawah kompetitor terendah) dan catat status LISTED di Tracker.",
			},
			BatchesScheduled: []int64{},
		},
		{
			DayNumber: 4,
			Title:     "Day 4: Re-Batching Gelombang 2",
			Focus:     "Perputaran Arus Kas",
			Tasks: []string{
				"Gunakan arus kas dari penjualan cepat (Fast Cashflow) untuk restock bahan baku.",
				"Mulai batch produksi gelombang 2 pada slot manufaktur yang sudah kosong.",
			},
			BatchesScheduled: []int64{},
		},
		{
			DayNumber: 5,
			Title:     "Day 5: Undercut Radar & Relist Aman",
			Focus:     "Proteksi Margin",
			Tasks: []string{
				"Periksa Sell Order yang tersalip pesaing (Undercut Radar).",
				"Gunakan Safe Relist Price sebelum menurunkan harga agar tidak merugi karena Broker & Relist Fee.",
			},
			BatchesScheduled: []int64{},
		},
		{
			DayNumber: 6,
			Title:     "Day 6: Puncak Trafik Akhir Pekan",
			Focus:     "Maksimalkan Volume Jual",
			Tasks: []string{
				"Pastikan seluruh stok siap jual sudah berada di Jita menjelang puncak pemain akhir pekan.",
				"Jalankan batch berdurasi panjang (High ROI / Ships / T2) agar slot tetap produktif saat offline.",
			},
			BatchesScheduled: []int64{},
		},
		{
			DayNumber: 7,
			Title:     "Day 7: Panen Profit & Audit Mingguan",
			Focus:     "Evaluasi & Naik Kelas Modal",
			Tasks: []string{
				"Tandai batch yang laku menjadi SOLD di Tracker dan hitung Realized Profit minggu ini.",
				"Evaluasi pertumbuhan modal terhadap proyeksi Minggu 1–4 dan siapkan alokasi kas minggu depan.",
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
		return "Tier 4 — Industrial Scale (T2 Hulls & Regional)", "Modal siap ekspansi ke T2 Cruisers/Battleships, reaksi bulan, atau arbitrase multi-hub."
	case capital >= Tier3CapitalThresholdISK:
		return "Tier 3 — T2 Modules & Invention", "Kas cukup untuk mulai Invention T2 (Modules/Drones/Ammo) dengan margin lebih tebal."
	case capital >= Tier2CapitalThresholdISK:
		return "Tier 2 — T1 Frigates, Destroyers & Rigs", "Naik kelas ke kapal T1 PvP/Faction Warfare dan Rig populer dengan volume harian tinggi."
	default:
		return "Tier 1 — Fast Cashflow (Ammo, Drones & Basic Modules)", "Fokus pada perputaran modal cepat dan jaga Cash Buffer 15% untuk pajak & broker fee."
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
