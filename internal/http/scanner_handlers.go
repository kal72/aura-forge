package http

import (
	"net/http"
	"strings"

	"aura-forge/internal/service"
)

// HandleScannerPage renders the Opportunity Scanner page (Manufacturing, Station Trading, Inter-Hub Arbitrage).
func (h *Handlers) HandleScannerPage(w http.ResponseWriter, r *http.Request) {
	mode := h.svc.GetModeInfo(r.Context())
	settings, _ := h.svc.GetSettings(r.Context())
	if h.renderer == nil {
		h.writeJSON(w, http.StatusOK, map[string]any{"status": "scanner_ready"})
		return
	}
	_ = h.renderer.RenderScanner(w, r, mode, *settings)
}

// HandleScanManufacturing runs a live multi-hub SDE Blueprint Manufacturing Margin Scan via Public ESI.
func (h *Handlers) HandleScanManufacturing(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	// Blank inputs default to Auto mode: Runs=0 (auto-sized), ME=-1 / TE=-1 (auto by Tech Level / Owned BPO)
	runs := 0
	if rawRuns := strings.TrimSpace(r.FormValue("runs")); rawRuns != "" {
		runs = parseIntDefault(rawRuns, 0)
	}
	me := -1
	if rawME := strings.TrimSpace(r.FormValue("me")); rawME != "" {
		me = parseIntDefault(rawME, -1)
	}
	te := -1
	if rawTE := strings.TrimSpace(r.FormValue("te")); rawTE != "" {
		te = parseIntDefault(rawTE, -1)
	}

	req := service.ManufacturingScanRequest{
		HubKey:              strings.TrimSpace(r.FormValue("hub")),
		MaterialHubKey:      strings.TrimSpace(r.FormValue("material_hub")),
		Category:            strings.TrimSpace(r.FormValue("category")),
		TechLevel:           parseIntDefault(r.FormValue("tech_level"), 0),
		SearchQuery:         strings.TrimSpace(r.FormValue("q")),
		Runs:                runs,
		ME:                  me,
		TE:                  te,
		StructureKey:        strings.TrimSpace(r.FormValue("structure_key")),
		MaterialPriceSource: strings.TrimSpace(r.FormValue("material_price_source")),
		ProfitableOnly:      r.FormValue("profitable_only") == "on" || r.FormValue("profitable_only") == "true",
		FitsCapitalOnly:     r.FormValue("fits_capital_only") == "on" || r.FormValue("fits_capital_only") == "true",
		SortBy:              strings.TrimSpace(r.FormValue("sort_by")),
	}

	results, err := h.svc.ScanManufacturingOpportunitiesV2(r.Context(), req)
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	if h.renderer == nil {
		h.writeJSON(w, http.StatusOK, map[string]any{"results": results, "error": errMsg})
		return
	}
	mode := h.svc.GetModeInfo(r.Context())
	_ = h.renderer.RenderScannerManufacturing(w, r, mode.IsAdmin(), results, errMsg)
}

// HandleScanStationTrading runs a live multi-hub Buy->Sell Station Trading Spread Scan.
func (h *Handlers) HandleScanStationTrading(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	hubKey := strings.TrimSpace(r.FormValue("hub"))
	category := strings.TrimSpace(r.FormValue("category"))
	profitableOnly := r.FormValue("profitable_only") == "on" || r.FormValue("profitable_only") == "true"

	results, err := h.svc.ScanStationTradingOpportunitiesV2(r.Context(), hubKey, category, profitableOnly)
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	if h.renderer == nil {
		h.writeJSON(w, http.StatusOK, map[string]any{"results": results, "error": errMsg})
		return
	}
	_ = h.renderer.RenderScannerStationTrading(w, r, results, errMsg)
}

// HandleScanArbitrage runs a live parallel Inter-Hub Arbitrage Scan with cargo cap & gank-safe trip sizing.
func (h *Handlers) HandleScanArbitrage(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	originKey := strings.TrimSpace(r.FormValue("origin_hub"))
	destKey := strings.TrimSpace(r.FormValue("dest_hub"))
	if originKey == "" {
		originKey = "jita"
	}
	if destKey == "" {
		destKey = "amarr"
	}
	category := strings.TrimSpace(r.FormValue("category"))
	cargoCap := parseFloatForm(r, "cargo_cap_m3", 5000)
	tripBudget := parseFloatForm(r, "trip_budget_isk", 150_000_000)
	profitableOnly := r.FormValue("profitable_only") == "on" || r.FormValue("profitable_only") == "true"

	results, err := h.svc.ScanInterHubArbitrageOpportunitiesV2(r.Context(), originKey, destKey, category, cargoCap, tripBudget, profitableOnly)
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	if h.renderer == nil {
		h.writeJSON(w, http.StatusOK, map[string]any{"results": results, "error": errMsg})
		return
	}
	_ = h.renderer.RenderScannerArbitrage(w, r, results, errMsg)
}

// HandleScanStockouts runs a live Local Hub Stockout & Thin-Supply Detector scan via Public ESI.
func (h *Handlers) HandleScanStockouts(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	hubKey := strings.TrimSpace(r.FormValue("hub"))
	category := strings.TrimSpace(r.FormValue("category"))

	results, err := h.svc.ScanLocalStockoutOpportunities(r.Context(), hubKey, category)
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	if h.renderer == nil {
		h.writeJSON(w, http.StatusOK, map[string]any{"results": results, "error": errMsg})
		return
	}
	_ = h.renderer.RenderScannerStockouts(w, r, results, errMsg)
}
