package http

import (
	"net/http"
	"strings"
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

// HandleScanManufacturing runs a live SDE Blueprint Manufacturing Margin Scan against Jita Public ESI prices.
func (h *Handlers) HandleScanManufacturing(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	category := strings.TrimSpace(r.FormValue("category"))
	runs := parseIntDefault(r.FormValue("runs"), 10)
	me := parseIntDefault(r.FormValue("me"), 10)
	te := parseIntDefault(r.FormValue("te"), 20)
	structureKey := strings.TrimSpace(r.FormValue("structure_key"))
	matSource := strings.TrimSpace(r.FormValue("material_price_source"))

	results, err := h.svc.ScanManufacturingOpportunities(r.Context(), category, runs, me, te, structureKey, matSource)
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	if h.renderer == nil {
		h.writeJSON(w, http.StatusOK, map[string]any{"results": results, "error": errMsg})
		return
	}
	_ = h.renderer.RenderScannerManufacturing(w, r, results, errMsg)
}

// HandleScanStationTrading runs a live Jita 4-4 Buy->Sell Station Trading Spread Scan.
func (h *Handlers) HandleScanStationTrading(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	category := strings.TrimSpace(r.FormValue("category"))
	results, err := h.svc.ScanStationTradingOpportunities(r.Context(), category)
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

// HandleScanArbitrage runs a live Inter-Hub Arbitrage Scan between two Empire trade hubs.
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

	results, err := h.svc.ScanInterHubArbitrageOpportunities(r.Context(), originKey, destKey)
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
