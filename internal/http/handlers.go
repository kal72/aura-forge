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
	RenderCatalog(w http.ResponseWriter, r *http.Request, mode service.ModeInfo, hub domain.TradeHub, items []domain.MarketItem, summary service.CatalogSummary, category, status, search, sortBy string, importRes *domain.ImportResult) error
	RenderCatalogTable(w http.ResponseWriter, r *http.Request, hub domain.TradeHub, items []domain.MarketItem) error
	RenderCatalogRecommendations(w http.ResponseWriter, r *http.Request, res *service.CatalogRecommendResult) error
	RenderCalculator(w http.ResponseWriter, r *http.Request, mode service.ModeInfo, settings domain.UserSettings, input domain.ProfitCalcInput, result *domain.ProfitCalcResult, calcErr string, blueprints []domain.SDEBlueprint) error
	RenderCalculatorResult(w http.ResponseWriter, r *http.Request, input domain.ProfitCalcInput, result *domain.ProfitCalcResult, calcErr string) error
	RenderBlueprintResult(w http.ResponseWriter, r *http.Request, result *domain.BlueprintCalcResult, errMsg string) error
	RenderAllocator(w http.ResponseWriter, r *http.Request, mode service.ModeInfo, cfg domain.AllocationConfig, result *domain.AllocationResult, allocErr string) error
	RenderAllocatorResult(w http.ResponseWriter, r *http.Request, result *domain.AllocationResult, allocErr string) error
	RenderPlanner(w http.ResponseWriter, r *http.Request, mode service.ModeInfo, data *service.PlannerViewData) error
	RenderTracker(w http.ResponseWriter, r *http.Request, mode service.ModeInfo, batches []domain.BatchRecord, statusFilter string) error
	RenderRelistResult(w http.ResponseWriter, r *http.Request, res *domain.RelistGuardResult, errMsg string) error
	RenderSettings(w http.ResponseWriter, r *http.Request, mode service.ModeInfo, settings domain.UserSettings, sdeStatus service.SDEImportStatus, saved bool) error
	RenderSDEStatusPartial(w http.ResponseWriter, r *http.Request, status service.SDEImportStatus) error
	RenderMineralTicker(w http.ResponseWriter, r *http.Request, pulse service.MarketPulseResponse) error
	RenderLogin(w http.ResponseWriter, r *http.Request, errMsg string, username string) error
	RenderUsers(w http.ResponseWriter, r *http.Request, mode service.ModeInfo, users []domain.AppUser, flashMsg string, flashErr string) error
	RenderMarketplace(w http.ResponseWriter, r *http.Request, mode service.ModeInfo, query string, hubKey string, presets []service.MarketplacePreset, snap *domain.MarketplaceSnapshot, errMsg string) error
	RenderMarketplaceResults(w http.ResponseWriter, r *http.Request, snap *domain.MarketplaceSnapshot, errMsg string) error
	RenderScanner(w http.ResponseWriter, r *http.Request, mode service.ModeInfo, settings domain.UserSettings) error
	RenderScannerManufacturing(w http.ResponseWriter, r *http.Request, results []domain.ManufacturingOpportunity, errMsg string) error
	RenderScannerStationTrading(w http.ResponseWriter, r *http.Request, results []domain.StationTradeOpportunity, errMsg string) error
	RenderScannerArbitrage(w http.ResponseWriter, r *http.Request, results []domain.ArbitrageOpportunity, errMsg string) error
	RenderWealth(w http.ResponseWriter, r *http.Request, mode service.ModeInfo, summary *domain.WealthSummary) error
	RenderInvention(w http.ResponseWriter, r *http.Request, mode service.ModeInfo, settings domain.UserSettings) error
	RenderInventionResult(w http.ResponseWriter, r *http.Request, res *domain.InventionOptimizationResult, errMsg string) error
	RenderMakeVsBuyResult(w http.ResponseWriter, r *http.Request, res *domain.MakeVsBuyResult, errMsg string) error
	RenderWiki(w http.ResponseWriter, r *http.Request, mode service.ModeInfo) error
	RenderPI(w http.ResponseWriter, r *http.Request, mode service.ModeInfo, items []domain.PICommodity, selectedTree *domain.PIRecipeAnalysis, colonies []domain.PIColony) error
	RenderPITreePartial(w http.ResponseWriter, r *http.Request, tree *domain.PIRecipeAnalysis, errMsg string) error
	RenderPIScanPartial(w http.ResponseWriter, r *http.Request, rows []domain.PIProfitRow, tier string, hub domain.TradeHub, errMsg string) error
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
	sortBy := r.URL.Query().Get("sort")

	hub := h.svc.ActiveCatalogHub(r.Context())
	items, summary, err := h.svc.ListCatalogSorted(r.Context(), category, status, search, sortBy)
	if err != nil {
		http.Error(w, "Failed to load catalog: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if h.renderer != nil {
		if IsHTMX(r) && r.Header.Get("HX-Target") == "catalog-table" {
			_ = h.renderer.RenderCatalogTable(w, r, hub, items)
			return
		}
		mode := h.svc.GetModeInfo(r.Context())
		_ = h.renderer.RenderCatalog(w, r, mode, hub, items, summary, category, status, search, sortBy, nil)
		return
	}
	h.writeJSON(w, http.StatusOK, items)
}

// HandleSetCatalogHub switches the user's Primary Trade Hub (Jita/Amarr/Dodixie/Rens/Hek)
// and immediately refreshes catalog prices & SDE BOM costs for the newly selected hub.
func (h *Handlers) HandleSetCatalogHub(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form payload", http.StatusBadRequest)
		return
	}
	hub := domain.LookupTradeHub(r.FormValue("hub"))
	settings, _ := h.svc.GetSettings(r.Context())
	if settings == nil {
		def := domain.DefaultUserSettings()
		settings = &def
	}
	settings.DefaultTradeHub = hub.Key
	if err := h.svc.UpdateSettings(r.Context(), settings); err != nil {
		if IsHTMX(r) {
			TriggerToast(w, "error", "Failed to save Trade Hub: "+err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	updated, _ := h.svc.SyncJitaPrices(r.Context())
	if IsHTMX(r) {
		if updated > 0 {
			TriggerToast(w, "success", fmt.Sprintf("Primary Trade Hub switched to %s (%d catalog items synced).", hub.Name, updated))
		} else {
			TriggerToast(w, "success", fmt.Sprintf("Primary Trade Hub switched to %s.", hub.Name))
		}
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/catalog", http.StatusSeeOther)
}

// HandleSaveCatalogItem creates or updates an item in the catalog.
func (h *Handlers) HandleSaveCatalogItem(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form payload", http.StatusBadRequest)
		return
	}

	var id int64
	if rawID := strings.TrimSpace(r.FormValue("id")); rawID != "" {
		if parsed, err := strconv.ParseInt(rawID, 10, 64); err == nil && parsed > 0 {
			id = parsed
		}
	}

	item := &domain.MarketItem{
		ID:                    id,
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
			TriggerToast(w, "error", "Failed to save item: "+err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if IsHTMX(r) {
		action := "saved"
		if id > 0 {
			action = "updated"
		}
		TriggerToast(w, "success", fmt.Sprintf("Item %s %s (%s).", saved.Name, action, saved.TrapAnalysis.Status))
		if id > 0 {
			w.Header().Set("HX-Refresh", "true")
			w.WriteHeader(http.StatusOK)
			return
		}
		items, _, _ := h.svc.ListCatalogSorted(r.Context(), "", "", "", "")
		hub := h.svc.ActiveCatalogHub(r.Context())
		if h.renderer != nil {
			_ = h.renderer.RenderCatalogTable(w, r, hub, items)
			return
		}
	}
	http.Redirect(w, r, "/catalog", http.StatusSeeOther)
}

// HandleDeleteCatalogItem deletes an item by ID.
func (h *Handlers) HandleDeleteCatalogItem(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "Invalid item ID", http.StatusBadRequest)
		return
	}
	if err := h.svc.DeleteCatalogItem(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if IsHTMX(r) {
		TriggerToast(w, "info", "Item removed from catalog.")
		items, _, _ := h.svc.ListCatalogSorted(r.Context(), "", "", "", "")
		hub := h.svc.ActiveCatalogHub(r.Context())
		if h.renderer != nil {
			_ = h.renderer.RenderCatalogTable(w, r, hub, items)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

// HandleBulkImport parses pasted TSV/CSV table data and enriches with Public ESI if enabled.
func (h *Handlers) HandleBulkImport(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form payload", http.StatusBadRequest)
		return
	}
	rawText := r.FormValue("bulk_text")
	enrichESI := r.FormValue("enrich_esi") == "on" || r.FormValue("enrich_esi") == "true"

	res, err := h.svc.ImportBulkItems(r.Context(), rawText, enrichESI)
	if err != nil {
		if IsHTMX(r) {
			TriggerToast(w, "error", "Bulk import failed: "+err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if IsHTMX(r) {
		TriggerToast(w, "success", fmt.Sprintf("Import complete: %d succeeded, %d errors.", res.SuccessCount, res.ErrorCount))
		items, summary, _ := h.svc.ListCatalogSorted(r.Context(), "", "", "", "")
		hub := h.svc.ActiveCatalogHub(r.Context())
		if h.renderer != nil {
			mode := h.svc.GetModeInfo(r.Context())
			_ = h.renderer.RenderCatalog(w, r, mode, hub, items, summary, "", "", "", "", res)
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

// HandleSyncJitaPrices triggers a scoped Jita price refresh (Catalog + PI P1-P4 + SDE Blueprints) via Public ESI.
func (h *Handlers) HandleSyncJitaPrices(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.SyncScopedJitaPrices(r.Context())
	if err != nil {
		if IsHTMX(r) {
			TriggerToast(w, "error", "Public ESI Jita sync failed: "+err.Error())
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	if res.SkippedBusy {
		if IsHTMX(r) {
			TriggerToast(w, "info", "A Jita price sync cycle is already running in the background.")
			w.WriteHeader(http.StatusOK)
			return
		}
		h.writeJSON(w, http.StatusOK, res)
		return
	}

	if IsHTMX(r) {
		TriggerToast(w, "success", fmt.Sprintf("Jita prices synced: %d/%d scoped items warmed (%d catalog updated) in %dms.", res.WarmedCount, res.TotalTypeIDs, res.CatalogUpdated, res.DurationMs))
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	h.writeJSON(w, http.StatusOK, res)
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

// HandleCatalogRecommend scans curated beginner-safe SDE blueprints against Available Capital and live Jita prices.
func (h *Handlers) HandleCatalogRecommend(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.RecommendCatalogForCapital(r.Context())
	if err != nil {
		if IsHTMX(r) {
			TriggerToast(w, "error", "Failed to build recommendations: "+err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if h.renderer != nil {
		_ = h.renderer.RenderCatalogRecommendations(w, r, res)
		return
	}
	h.writeJSON(w, http.StatusOK, res)
}

// HandleCatalogRecommendApply adds one or all viable recommended starter items into the user's Catalog.
func (h *Handlers) HandleCatalogRecommendApply(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form payload", http.StatusBadRequest)
		return
	}
	mode := strings.TrimSpace(r.FormValue("mode")) // "all_viable" or "single"
	targetTypeID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("type_id")), 10, 32)

	recRes, err := h.svc.RecommendCatalogForCapital(r.Context())
	if err != nil {
		if IsHTMX(r) {
			TriggerToast(w, "error", "Failed to apply recommendations: "+err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	added := 0
	for _, rec := range recRes.Items {
		if mode == "single" {
			if rec.TypeID != int32(targetTypeID) {
				continue
			}
		} else {
			if rec.AlreadyInCatalog || rec.TrapStatus != domain.TrapStatusLayak {
				continue
			}
		}
		sellPrice := rec.RecommendedSell
		if sellPrice <= 0 {
			sellPrice = rec.JitaSell
		}
		item := &domain.MarketItem{
			TypeID:                rec.TypeID,
			Name:                  rec.Name,
			Category:              rec.Category,
			EstimatedJitaSell:     sellPrice,
			EstimatedJitaBuy:      rec.JitaBuy,
			DailyVolume:           rec.DailyVolume,
			MaterialCost:          rec.MaterialCost,
			BatchOutputQty:        rec.BatchOutputQty,
			IsManufacturable:      true,
			IsFrequentlyDestroyed: rec.IsFrequentlyDestroyed,
			TrapAnalysis: domain.TrapReport{
				Avg7dPrice:       rec.Avg7d,
				BOMSyncedFromSDE: true,
			},
		}
		if _, err := h.svc.SaveCatalogItem(r.Context(), item); err == nil {
			added++
		}
	}

	if IsHTMX(r) {
		if added == 0 {
			TriggerToast(w, "info", "All recommended VIABLE items are already in your catalog.")
		} else {
			TriggerToast(w, "success", fmt.Sprintf("Added %d recommended item(s) to your Catalog.", added))
		}
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/catalog", http.StatusSeeOther)
}

// HandleLookupItemESI resolves an item name and returns live sell/buy price, daily volume,
// 7d low/avg/high range, break-even floor, Recommended Sell Price, and optional SDE BOM material cost
// at the requested hub (or the user's configured Primary Trade Hub).
func (h *Handlers) HandleLookupItemESI(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		http.Error(w, "name query parameter is required", http.StatusBadRequest)
		return
	}
	hub := h.svc.ActiveCatalogHub(r.Context())
	if rawHub := strings.TrimSpace(r.URL.Query().Get("hub")); rawHub != "" {
		hub = domain.LookupTradeHub(rawHub)
	}

	jp, dailyVol, err := h.svc.LookupItemPriceForHub(r.Context(), hub, name)
	if err != nil {
		h.writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}

	batchQty := parseIntDefault(r.URL.Query().Get("batch_qty"), 10)
	sdeMatCost, hasSDE := h.svc.EstimateSDEMaterialCostForHub(r.Context(), hub, jp.TypeID, batchQty, nil)
	pi := h.svc.GetItemPriceIntelligenceForHub(r.Context(), hub, jp.TypeID, jp.LowestSell, jp.HighestBuy, sdeMatCost, batchQty)

	h.writeJSON(w, http.StatusOK, map[string]any{
		"hub_key":                hub.Key,
		"hub_name":               hub.Name,
		"type_id":                jp.TypeID,
		"lowest_sell":            jp.LowestSell,
		"highest_buy":            jp.HighestBuy,
		"daily_volume":           dailyVol,
		"sde_has_blueprint":      hasSDE,
		"sde_material_cost":      sdeMatCost,
		"low_7d":                 pi.Low7d,
		"avg_7d":                 pi.Avg7d,
		"high_7d":                pi.High7d,
		"break_even_price":       pi.BreakEvenPrice,
		"recommended_sell_price": pi.RecommendedSellPrice,
		"recommendation_note":    pi.RecommendationNote,
		"price_regime":           pi.PriceRegime,
	})
}

// HandleLiveMinerals returns live trade-hub Market Pulse quotes (Minerals, PI Benchmarks, Macro & Fuel, Hot Blueprints, Global Movers) from Public ESI.
func (h *Handlers) HandleLiveMinerals(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	hubKey := r.URL.Query().Get("hub")
	sortBy := r.URL.Query().Get("sort")
	pulse := h.svc.GetLiveMarketPulseView(r.Context(), category, hubKey, sortBy)
	if h.renderer != nil && IsHTMX(r) {
		_ = h.renderer.RenderMineralTicker(w, r, pulse)
		return
	}
	h.writeJSON(w, http.StatusOK, pulse)
}

// HandleCheckUndercuts compares all LISTED/SLOW_MOVER batches against live Jita lowest sell prices via Public ESI.
func (h *Handlers) HandleCheckUndercuts(w http.ResponseWriter, r *http.Request) {
	checked, undercuts, err := h.svc.CheckTrackerUndercutsESI(r.Context())
	if err != nil {
		if IsHTMX(r) {
			TriggerToast(w, "error", "Cek Undercut ESI gagal: "+err.Error())
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	if IsHTMX(r) {
		level := "success"
		msg := fmt.Sprintf("Public ESI Undercut Radar: %d batch dicek, aman (0 undercut).", checked)
		if undercuts > 0 {
			level = "error"
			msg = fmt.Sprintf("PERHATIAN: %d dari %d batch LISTED sedang ter-undercut di Jita!", undercuts, checked)
		}
		TriggerToast(w, level, msg)
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]any{"checked": checked, "undercuts": undercuts})
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
		blueprints, _ := h.svc.SearchBlueprints(r.Context(), "", "", 100)
		_ = h.renderer.RenderCalculator(w, r, mode, *settings, defaultIn, &res, errMsg, blueprints)
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
		sdeStatus := h.svc.GetSDEImportStatus(r.Context())
		_ = h.renderer.RenderSettings(w, r, mode, *settings, sdeStatus, saved)
		return
	}
	h.writeJSON(w, http.StatusOK, settings)
}

// HandleSaveSettings persists user tax, slot, capital, and background price updater parameters.
func (h *Handlers) HandleSaveSettings(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form submission", http.StatusBadRequest)
		return
	}

	existing, _ := h.svc.GetSettings(r.Context())
	if existing == nil {
		def := domain.DefaultUserSettings()
		existing = &def
	}

	cfg := *existing
	cfg.ID = 1

	// Support saving either the Tax/Capital form or the Background Price Updater form independently.
	if r.FormValue("form_section") == "price_updater" {
		cfg.AutoPriceSyncEnabled = r.FormValue("auto_price_sync_enabled") == "on" || r.FormValue("auto_price_sync_enabled") == "true"
		cfg.AutoPriceSyncIntervalMinutes = domain.ClampPriceSyncInterval(parseIntForm(r, "auto_price_sync_interval_minutes", existing.AutoPriceSyncIntervalMinutes))
	} else {
		cfg.StartingCapital = parseFloatForm(r, "starting_capital", existing.StartingCapital)
		cfg.BrokerFeePercent = parseFloatForm(r, "broker_fee_percent", existing.BrokerFeePercent)
		cfg.SalesTaxPercent = parseFloatForm(r, "sales_tax_percent", existing.SalesTaxPercent)
		cfg.RelistFeePercent = parseFloatForm(r, "relist_fee_percent", existing.RelistFeePercent)
		cfg.ManufacturingSlots = parseIntForm(r, "manufacturing_slots", existing.ManufacturingSlots)
		cfg.ScienceSlots = parseIntForm(r, "science_slots", existing.ScienceSlots)
		cfg.SystemCostIndex = parseFloatForm(r, "system_cost_index", existing.SystemCostIndex)
		cfg.FreightCostPerM3 = parseFloatForm(r, "freight_cost_per_m3", existing.FreightCostPerM3)
		cfg.FacilityTaxPercent = parseFloatForm(r, "facility_tax_percent", existing.FacilityTaxPercent)
		if rawHub := strings.TrimSpace(r.FormValue("default_trade_hub")); rawHub != "" {
			cfg.DefaultTradeHub = domain.LookupTradeHub(rawHub).Key
		}
		if r.Form.Has("auto_price_sync_interval_minutes") {
			cfg.AutoPriceSyncEnabled = r.FormValue("auto_price_sync_enabled") == "on" || r.FormValue("auto_price_sync_enabled") == "true"
			cfg.AutoPriceSyncIntervalMinutes = domain.ClampPriceSyncInterval(parseIntForm(r, "auto_price_sync_interval_minutes", existing.AutoPriceSyncIntervalMinutes))
		}
	}

	if err := h.svc.UpdateSettings(r.Context(), &cfg); err != nil {
		if IsHTMX(r) {
			TriggerToast(w, "error", "Failed to save settings: "+err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if IsHTMX(r) {
		TriggerToast(w, "success", "Settings saved to PostgreSQL.")
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
