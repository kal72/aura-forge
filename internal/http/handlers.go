package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"aura-forge/internal/domain"
	"aura-forge/internal/service"

	"github.com/go-chi/chi/v5"
)

// Renderer defines the UI rendering contract implemented by Templ views in internal/web.
type Renderer interface {
	RenderDashboard(w http.ResponseWriter, r *http.Request, data *service.DashboardSummary) error
	RenderCatalog(w http.ResponseWriter, r *http.Request, mode service.ModeInfo, items []domain.MarketItem, category, status, search string, importRes *domain.ImportResult) error
	RenderCatalogTable(w http.ResponseWriter, r *http.Request, items []domain.MarketItem) error
	RenderCalculator(w http.ResponseWriter, r *http.Request, mode service.ModeInfo, settings domain.UserSettings, input domain.ProfitCalcInput, result *domain.ProfitCalcResult, calcErr string) error
	RenderCalculatorResult(w http.ResponseWriter, r *http.Request, input domain.ProfitCalcInput, result *domain.ProfitCalcResult, calcErr string) error
	RenderAllocator(w http.ResponseWriter, r *http.Request, mode service.ModeInfo, cfg domain.AllocationConfig, result *domain.AllocationResult, allocErr string) error
	RenderAllocatorResult(w http.ResponseWriter, r *http.Request, result *domain.AllocationResult, allocErr string) error
	RenderPlanner(w http.ResponseWriter, r *http.Request, mode service.ModeInfo, data *service.PlannerViewData) error
	RenderTracker(w http.ResponseWriter, r *http.Request, mode service.ModeInfo, batches []domain.BatchRecord, statusFilter string) error
	RenderRelistResult(w http.ResponseWriter, r *http.Request, res *domain.RelistGuardResult, errMsg string) error
	RenderSettings(w http.ResponseWriter, r *http.Request, mode service.ModeInfo, settings domain.UserSettings, saved bool) error
}

// Handlers exposes all HTTP and HTMX endpoints for aura-forge.
type Handlers struct {
	svc      *service.CopilotService
	renderer Renderer
}

func NewHandlers(svc *service.CopilotService, renderer Renderer) *Handlers {
	return &Handlers{svc: svc, renderer: renderer}
}

func (h *Handlers) writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// HandleHealth returns service liveness and current ESI mode.
func (h *Handlers) HandleHealth(w http.ResponseWriter, r *http.Request) {
	mode := h.svc.GetModeInfo(r.Context())
	h.writeJSON(w, http.StatusOK, map[string]any{
		"status":        "ok",
		"app":           "aura-forge",
		"mode":          mode.ModeLabel,
		"sso_enabled":   mode.SSOEnabled,
		"sso_connected": mode.SSOConnected,
	})
}

// HandleDashboard renders the main KPI dashboard.
func (h *Handlers) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	summary, err := h.svc.GetDashboard(r.Context())
	if err != nil {
		http.Error(w, "Gagal memuat dashboard: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if h.renderer != nil {
		_ = h.renderer.RenderDashboard(w, r, summary)
		return
	}
	h.writeJSON(w, http.StatusOK, summary)
}

// HandleCatalog renders the item catalog page or HTMX table partial.
func (h *Handlers) HandleCatalog(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	status := r.URL.Query().Get("status")
	search := r.URL.Query().Get("q")

	items, err := h.svc.ListCatalog(r.Context(), category, status, search)
	if err != nil {
		http.Error(w, "Gagal memuat catalog: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if h.renderer != nil {
		if IsHTMX(r) && r.Header.Get("HX-Target") == "catalog-table" {
			_ = h.renderer.RenderCatalogTable(w, r, items)
			return
		}
		mode := h.svc.GetModeInfo(r.Context())
		_ = h.renderer.RenderCatalog(w, r, mode, items, category, status, search, nil)
		return
	}
	h.writeJSON(w, http.StatusOK, items)
}

// HandleSaveCatalogItem creates or updates an item in the catalog.
func (h *Handlers) HandleSaveCatalogItem(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Form tidak valid", http.StatusBadRequest)
		return
	}

	item := &domain.MarketItem{
		Name:                  strings.TrimSpace(r.FormValue("name")),
		Category:              domain.ItemCategory(r.FormValue("category")),
		EstimatedJitaSell:     parseFloatForm(r, "estimated_jita_sell", 0),
		EstimatedJitaBuy:      parseFloatForm(r, "estimated_jita_buy", 0),
		DailyVolume:           parseFloatForm(r, "daily_volume", 0),
		MaterialCost:          parseFloatForm(r, "material_cost", 0),
		BatchOutputQty:        parseIntForm(r, "batch_output_qty", 1),
		ProductionTimeSeconds: parseIntForm(r, "production_time_hours", 4) * 3600,
		ExtraSetupCost:        parseFloatForm(r, "extra_setup_cost", 0),
		IsManufacturable:      r.FormValue("is_manufacturable") != "false",
		RequiresRareBPC:       r.FormValue("requires_rare_bpc") == "true" || r.FormValue("requires_rare_bpc") == "on",
		IsFrequentlyDestroyed: r.FormValue("is_frequently_destroyed") == "true" || r.FormValue("is_frequently_destroyed") == "on",
	}
	if item.Category == "" {
		item.Category = domain.CategoryOther
	}

	saved, err := h.svc.SaveCatalogItem(r.Context(), item)
	if err != nil {
		if IsHTMX(r) {
			TriggerToast(w, "error", "Gagal menyimpan item: "+err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if IsHTMX(r) {
		TriggerToast(w, "success", fmt.Sprintf("Item %s disimpan (%s).", saved.Name, saved.TrapAnalysis.Status))
		items, _ := h.svc.ListCatalog(r.Context(), "", "", "")
		if h.renderer != nil {
			_ = h.renderer.RenderCatalogTable(w, r, items)
			return
		}
	}
	http.Redirect(w, r, "/catalog", http.StatusSeeOther)
}

// HandleDeleteCatalogItem deletes an item by ID.
func (h *Handlers) HandleDeleteCatalogItem(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "ID item tidak valid", http.StatusBadRequest)
		return
	}
	if err := h.svc.DeleteCatalogItem(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if IsHTMX(r) {
		TriggerToast(w, "info", "Item dihapus dari catalog.")
		items, _ := h.svc.ListCatalog(r.Context(), "", "", "")
		if h.renderer != nil {
			_ = h.renderer.RenderCatalogTable(w, r, items)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

// HandleBulkImport parses pasted TSV/CSV table data and enriches with Public ESI if enabled.
func (h *Handlers) HandleBulkImport(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Form tidak valid", http.StatusBadRequest)
		return
	}
	rawText := r.FormValue("bulk_text")
	enrichESI := r.FormValue("enrich_esi") == "on" || r.FormValue("enrich_esi") == "true"

	res, err := h.svc.ImportBulkItems(r.Context(), rawText, enrichESI)
	if err != nil {
		if IsHTMX(r) {
			TriggerToast(w, "error", "Import gagal: "+err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if IsHTMX(r) {
		TriggerToast(w, "success", fmt.Sprintf("Import selesai: %d sukses, %d error.", res.SuccessCount, res.ErrorCount))
		items, _ := h.svc.ListCatalog(r.Context(), "", "", "")
		if h.renderer != nil {
			mode := h.svc.GetModeInfo(r.Context())
			_ = h.renderer.RenderCatalog(w, r, mode, items, "", "", "", res)
			return
		}
	}
	h.writeJSON(w, http.StatusOK, res)
}

// HandleLoadSampleData populates sample beginner items & trap items.
func (h *Handlers) HandleLoadSampleData(w http.ResponseWriter, r *http.Request) {
	count, err := h.svc.LoadSampleData(r.Context())
	if err != nil {
		if IsHTMX(r) {
			TriggerToast(w, "error", "Gagal memuat sample data: "+err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if IsHTMX(r) {
		TriggerToast(w, "success", fmt.Sprintf("%d item contoh (termasuk contoh Jebakan BPC & Slow Mover) berhasil dimuat.", count))
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/catalog", http.StatusSeeOther)
}

// HandleSyncJitaPrices fetches live Jita prices and volumes from Public ESI.
func (h *Handlers) HandleSyncJitaPrices(w http.ResponseWriter, r *http.Request) {
	count, err := h.svc.SyncJitaPrices(r.Context())
	if err != nil {
		if IsHTMX(r) {
			TriggerToast(w, "error", "Sync Public ESI gagal: "+err.Error())
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	if IsHTMX(r) {
		TriggerToast(w, "success", fmt.Sprintf("Harga Jita & volume harian %d item diperbarui dari Public ESI.", count))
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]any{"updated": count})
}

// HandleSyncSCI fetches live System Cost Index from Public ESI.
func (h *Handlers) HandleSyncSCI(w http.ResponseWriter, r *http.Request) {
	systemID := int32(parseIntForm(r, "solar_system_id", 30000142))
	sci, err := h.svc.SyncSystemCostIndex(r.Context(), systemID)
	if err != nil {
		if IsHTMX(r) {
			TriggerToast(w, "error", "Gagal ambil SCI dari Public ESI: "+err.Error())
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	if IsHTMX(r) {
		TriggerToast(w, "success", fmt.Sprintf("System Cost Index diperbarui: %.2f%%", sci*100))
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]any{"solar_system_id": systemID, "manufacturing_sci": sci})
}

// HandleLookupItemESI resolves an item name and returns its Jita sell/buy price & daily volume.
func (h *Handlers) HandleLookupItemESI(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		http.Error(w, "parameter name wajib diisi", http.StatusBadRequest)
		return
	}
	jp, dailyVol, err := h.svc.LookupItemPriceESI(r.Context(), name)
	if err != nil {
		h.writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]any{
		"type_id":      jp.TypeID,
		"lowest_sell":  jp.LowestSell,
		"highest_buy":  jp.HighestBuy,
		"daily_volume": dailyVol,
	})
}

// HandleCalculator renders the Net Profit, Tax & Break-Even Calculator page.
func (h *Handlers) HandleCalculator(w http.ResponseWriter, r *http.Request) {
	settings, _ := h.svc.GetSettings(r.Context())
	defaultIn := domain.ProfitCalcInput{
		ItemName:           "Hobgoblin I",
		OutputQty:          500,
		UnitSellPrice:      14_500,
		MaterialCost:       4_900_000,
		ExtraSetupCost:     0,
		SystemCostIndex:    settings.SystemCostIndex,
		FacilityTaxPercent: settings.FacilityTaxPercent,
		FreightCost:        50_000,
		BrokerFeePercent:   settings.BrokerFeePercent,
		SalesTaxPercent:    settings.SalesTaxPercent,
		RelistFeePercent:   settings.RelistFeePercent,
		EstimatedRelists:   1,
		BatchesPerWeek:     4,
	}
	res, err := h.svc.CalculateProfit(r.Context(), defaultIn)
	var errMsg string
	if err != nil {
		errMsg = err.Error()
	}

	if h.renderer != nil {
		mode := h.svc.GetModeInfo(r.Context())
		_ = h.renderer.RenderCalculator(w, r, mode, *settings, defaultIn, &res, errMsg)
		return
	}
	h.writeJSON(w, http.StatusOK, res)
}

// HandleCalculatorCompute computes profit in real-time via HTMX POST.
func (h *Handlers) HandleCalculatorCompute(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Form tidak valid", http.StatusBadRequest)
		return
	}
	settings, _ := h.svc.GetSettings(r.Context())
	in := domain.ProfitCalcInput{
		ItemName:           strings.TrimSpace(r.FormValue("item_name")),
		OutputQty:          parseIntForm(r, "output_qty", 1),
		UnitSellPrice:      parseFloatForm(r, "unit_sell_price", 0),
		MaterialCost:       parseFloatForm(r, "material_cost", 0),
		ExtraSetupCost:     parseFloatForm(r, "extra_setup_cost", 0),
		JobInstallCost:     parseFloatForm(r, "job_install_cost", 0),
		SystemCostIndex:    parseFloatForm(r, "system_cost_index", settings.SystemCostIndex),
		FacilityTaxPercent: parseFloatForm(r, "facility_tax_percent", settings.FacilityTaxPercent),
		FreightCost:        parseFloatForm(r, "freight_cost", 0),
		BrokerFeePercent:   parseFloatForm(r, "broker_fee_percent", settings.BrokerFeePercent),
		SalesTaxPercent:    parseFloatForm(r, "sales_tax_percent", settings.SalesTaxPercent),
		RelistFeePercent:   parseFloatForm(r, "relist_fee_percent", settings.RelistFeePercent),
		EstimatedRelists:   parseIntForm(r, "estimated_relists", 1),
		BatchesPerWeek:     parseIntForm(r, "batches_per_week", 1),
	}

	res, err := h.svc.CalculateProfit(r.Context(), in)
	var errMsg string
	var resPtr *domain.ProfitCalcResult
	if err != nil {
		errMsg = err.Error()
	} else {
		resPtr = &res
	}

	if h.renderer != nil {
		_ = h.renderer.RenderCalculatorResult(w, r, in, resPtr, errMsg)
		return
	}
	if errMsg != "" {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": errMsg})
		return
	}
	h.writeJSON(w, http.StatusOK, res)
}

// HandleAllocator renders the Smart Capital Allocator page.
func (h *Handlers) HandleAllocator(w http.ResponseWriter, r *http.Request) {
	cfg := domain.DefaultAllocationConfig()
	res, err := h.svc.RunCapitalAllocation(r.Context(), cfg)
	var errMsg string
	if err != nil {
		errMsg = err.Error()
	}

	if h.renderer != nil {
		mode := h.svc.GetModeInfo(r.Context())
		_ = h.renderer.RenderAllocator(w, r, mode, cfg, res, errMsg)
		return
	}
	h.writeJSON(w, http.StatusOK, res)
}

// HandleAllocatorRun recomputes the 4-bucket allocation via HTMX.
func (h *Handlers) HandleAllocatorRun(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	cfg := domain.AllocationConfig{
		FastCashflowPct: parseFloatForm(r, "fast_cashflow_pct", domain.DefaultFastCashflowPct),
		MidTierPct:      parseFloatForm(r, "mid_tier_pct", domain.DefaultMidTierPct),
		HighROIPct:      parseFloatForm(r, "high_roi_pct", domain.DefaultHighROIPct),
		CashBufferPct:   parseFloatForm(r, "cash_buffer_pct", domain.DefaultCashBufferPct),
	}

	res, err := h.svc.RunCapitalAllocation(r.Context(), cfg)
	var errMsg string
	if err != nil {
		errMsg = err.Error()
	}

	if h.renderer != nil {
		_ = h.renderer.RenderAllocatorResult(w, r, res, errMsg)
		return
	}
	if errMsg != "" {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": errMsg})
		return
	}
	h.writeJSON(w, http.StatusOK, res)
}

// HandleAllocatorToPlan converts the allocation into an active Weekly Plan.
func (h *Handlers) HandleAllocatorToPlan(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	cfg := domain.AllocationConfig{
		FastCashflowPct: parseFloatForm(r, "fast_cashflow_pct", domain.DefaultFastCashflowPct),
		MidTierPct:      parseFloatForm(r, "mid_tier_pct", domain.DefaultMidTierPct),
		HighROIPct:      parseFloatForm(r, "high_roi_pct", domain.DefaultHighROIPct),
		CashBufferPct:   parseFloatForm(r, "cash_buffer_pct", domain.DefaultCashBufferPct),
	}
	title := strings.TrimSpace(r.FormValue("title"))
	weekNum := parseIntForm(r, "week_number", 1)

	plan, err := h.svc.CreatePlanFromAllocation(r.Context(), title, weekNum, cfg)
	if err != nil {
		if IsHTMX(r) {
			TriggerToast(w, "error", "Gagal membuat Weekly Plan: "+err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if IsHTMX(r) {
		TriggerToast(w, "success", "Weekly Plan berhasil dibuat! Mengarahkan ke Planner...")
		w.Header().Set("HX-Redirect", fmt.Sprintf("/planner?plan_id=%d", plan.ID))
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/planner?plan_id=%d", plan.ID), http.StatusSeeOther)
}

// HandleAllocatorToBatches converts allocation directly into Draft Batches in Tracker.
func (h *Handlers) HandleAllocatorToBatches(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	cfg := domain.AllocationConfig{
		FastCashflowPct: parseFloatForm(r, "fast_cashflow_pct", domain.DefaultFastCashflowPct),
		MidTierPct:      parseFloatForm(r, "mid_tier_pct", domain.DefaultMidTierPct),
		HighROIPct:      parseFloatForm(r, "high_roi_pct", domain.DefaultHighROIPct),
		CashBufferPct:   parseFloatForm(r, "cash_buffer_pct", domain.DefaultCashBufferPct),
	}

	count, err := h.svc.CreateDraftBatchesFromAllocation(r.Context(), cfg)
	if err != nil {
		if IsHTMX(r) {
			TriggerToast(w, "error", "Gagal membuat Draft Batches: "+err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if IsHTMX(r) {
		TriggerToast(w, "success", fmt.Sprintf("%d Draft Batches berhasil dikirim ke Tracker.", count))
		w.Header().Set("HX-Redirect", "/tracker")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/tracker", http.StatusSeeOther)
}

// HandlePlanner renders the Weekly Production & Slot Planner page.
func (h *Handlers) HandlePlanner(w http.ResponseWriter, r *http.Request) {
	planID, _ := strconv.ParseInt(r.URL.Query().Get("plan_id"), 10, 64)
	data, err := h.svc.GetPlannerView(r.Context(), planID)
	if err != nil {
		http.Error(w, "Gagal memuat planner: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if h.renderer != nil {
		mode := h.svc.GetModeInfo(r.Context())
		_ = h.renderer.RenderPlanner(w, r, mode, data)
		return
	}
	h.writeJSON(w, http.StatusOK, data)
}

// HandlePlannerConvertBatches converts a weekly plan's items into Draft Batches in Tracker.
func (h *Handlers) HandlePlannerConvertBatches(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	planID, _ := strconv.ParseInt(r.FormValue("plan_id"), 10, 64)

	count, err := h.svc.ConvertPlanToTrackerBatches(r.Context(), planID)
	if err != nil {
		if IsHTMX(r) {
			TriggerToast(w, "error", "Konversi gagal: "+err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if IsHTMX(r) {
		TriggerToast(w, "success", fmt.Sprintf("%d batch berhasil dieksekusi ke Tracker (status DRAFT).", count))
		w.Header().Set("HX-Redirect", "/tracker")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/tracker", http.StatusSeeOther)
}

// HandleTracker renders the Production, Inventory & Market Order Tracker.
func (h *Handlers) HandleTracker(w http.ResponseWriter, r *http.Request) {
	statusFilter := r.URL.Query().Get("status")
	batches, err := h.svc.ListBatches(r.Context(), statusFilter)
	if err != nil {
		http.Error(w, "Gagal memuat tracker: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if h.renderer != nil {
		mode := h.svc.GetModeInfo(r.Context())
		_ = h.renderer.RenderTracker(w, r, mode, batches, statusFilter)
		return
	}
	h.writeJSON(w, http.StatusOK, batches)
}

// HandleSaveBatch creates a manual production batch in Tracker.
func (h *Handlers) HandleSaveBatch(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Form tidak valid", http.StatusBadRequest)
		return
	}
	batch := &domain.BatchRecord{
		ItemName:             strings.TrimSpace(r.FormValue("item_name")),
		Runs:                 parseIntForm(r, "runs", 1),
		UnitsOutput:          parseIntForm(r, "units_output", 1),
		TotalMaterialCost:    parseFloatForm(r, "total_material_cost", 0),
		ActualProductionCost: parseFloatForm(r, "actual_production_cost", 0),
		ActualSellPrice:      parseFloatForm(r, "actual_sell_price", 0),
		Status:               domain.BatchStatus(r.FormValue("status")),
	}
	if batch.Status == "" {
		batch.Status = domain.BatchStatusDraft
	}

	if _, err := h.svc.SaveBatch(r.Context(), batch); err != nil {
		if IsHTMX(r) {
			TriggerToast(w, "error", "Gagal menyimpan batch: "+err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if IsHTMX(r) {
		TriggerToast(w, "success", "Batch produksi ditambahkan ke Tracker.")
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/tracker", http.StatusSeeOther)
}

// HandleUpdateBatchStatus advances or updates a batch lifecycle status.
func (h *Handlers) HandleUpdateBatchStatus(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "ID batch tidak valid", http.StatusBadRequest)
		return
	}
	_ = r.ParseForm()
	newStatus := domain.BatchStatus(r.FormValue("status"))
	if newStatus == "" {
		http.Error(w, "status wajib diisi", http.StatusBadRequest)
		return
	}

	if err := h.svc.UpdateBatchStatus(r.Context(), id, newStatus); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if IsHTMX(r) {
		TriggerToast(w, "success", fmt.Sprintf("Status batch #%d diperbarui ke %s.", id, newStatus))
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/tracker", http.StatusSeeOther)
}

// HandleDeleteBatch removes a batch from Tracker.
func (h *Handlers) HandleDeleteBatch(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "ID batch tidak valid", http.StatusBadRequest)
		return
	}
	if err := h.svc.DeleteBatch(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if IsHTMX(r) {
		TriggerToast(w, "info", fmt.Sprintf("Batch #%d dihapus.", id))
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// HandleRelistCheck simulates a proposed sell price against the Relist & Undercut Guard.
func (h *Handlers) HandleRelistCheck(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "ID batch tidak valid", http.StatusBadRequest)
		return
	}
	_ = r.ParseForm()
	proposedPrice := parseFloatForm(r, "proposed_price", 0)

	res, checkErr := h.svc.CheckRelistGuard(r.Context(), id, proposedPrice)
	var errMsg string
	if checkErr != nil {
		errMsg = checkErr.Error()
	}

	if h.renderer != nil {
		_ = h.renderer.RenderRelistResult(w, r, res, errMsg)
		return
	}
	if errMsg != "" {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": errMsg})
		return
	}
	h.writeJSON(w, http.StatusOK, res)
}

// HandleSettings renders the Settings & EVE SSO status page.
func (h *Handlers) HandleSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.svc.GetSettings(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	saved := r.URL.Query().Get("saved") == "1"

	if h.renderer != nil {
		mode := h.svc.GetModeInfo(r.Context())
		_ = h.renderer.RenderSettings(w, r, mode, *settings, saved)
		return
	}
	h.writeJSON(w, http.StatusOK, settings)
}

// HandleSaveSettings persists user tax, slot, and capital parameters.
func (h *Handlers) HandleSaveSettings(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Form tidak valid", http.StatusBadRequest)
		return
	}
	cfg := &domain.UserSettings{
		ID:                 1,
		StartingCapital:    parseFloatForm(r, "starting_capital", 100_000_000),
		BrokerFeePercent:   parseFloatForm(r, "broker_fee_percent", 3.0),
		SalesTaxPercent:    parseFloatForm(r, "sales_tax_percent", 3.6),
		RelistFeePercent:   parseFloatForm(r, "relist_fee_percent", 0.5),
		ManufacturingSlots: parseIntForm(r, "manufacturing_slots", 10),
		ScienceSlots:       parseIntForm(r, "science_slots", 1),
		SystemCostIndex:    parseFloatForm(r, "system_cost_index", 0.045),
		FreightCostPerM3:   parseFloatForm(r, "freight_cost_per_m3", 300),
		FacilityTaxPercent: parseFloatForm(r, "facility_tax_percent", 0.25),
	}

	if err := h.svc.UpdateSettings(r.Context(), cfg); err != nil {
		if IsHTMX(r) {
			TriggerToast(w, "error", "Gagal menyimpan pengaturan: "+err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if IsHTMX(r) {
		TriggerToast(w, "success", "Pengaturan pajak, slot, dan modal berhasil disimpan.")
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/settings?saved=1", http.StatusSeeOther)
}

func parseFloatForm(r *http.Request, key string, fallback float64) float64 {
	raw := strings.TrimSpace(r.FormValue(key))
	if raw == "" {
		return fallback
	}
	val, err := strconv.ParseFloat(strings.ReplaceAll(raw, ",", ""), 64)
	if err != nil {
		return fallback
	}
	return val
}

func parseIntForm(r *http.Request, key string, fallback int) int {
	raw := strings.TrimSpace(r.FormValue(key))
	if raw == "" {
		return fallback
	}
	val, err := strconv.Atoi(strings.ReplaceAll(raw, ",", ""))
	if err != nil {
		return fallback
	}
	return val
}
