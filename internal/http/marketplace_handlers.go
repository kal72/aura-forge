package http

import (
	"net/http"
	"strconv"
	"strings"

	"aura-forge/internal/domain"
	"aura-forge/internal/service"
)

// HandleMarketplace renders the EVE Online Marketplace Browser page.
func (h *Handlers) HandleMarketplace(w http.ResponseWriter, r *http.Request) {
	mode := h.svc.GetModeInfo(r.Context())
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	hubKey := strings.TrimSpace(r.URL.Query().Get("hub"))
	if hubKey == "" {
		hubKey = "jita"
	}

	var (
		snap   *domain.MarketplaceSnapshot
		errMsg string
	)
	if query != "" {
		res, err := h.svc.GetMarketplaceSnapshot(r.Context(), query, hubKey, true)
		if err != nil {
			errMsg = err.Error()
		} else {
			snap = res
		}
	}

	if h.renderer == nil {
		h.writeJSON(w, http.StatusOK, map[string]any{"snapshot": snap, "error": errMsg})
		return
	}
	_ = h.renderer.RenderMarketplace(w, r, mode, query, hubKey, service.DefaultMarketplacePresets, snap, errMsg)
}

// HandleMarketplaceSearch handles HTMX live item lookup & multi-hub comparison.
func (h *Handlers) HandleMarketplaceSearch(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	hubKey := strings.TrimSpace(r.URL.Query().Get("hub"))
	if hubKey == "" {
		hubKey = "jita"
	}

	snap, err := h.svc.GetMarketplaceSnapshot(r.Context(), query, hubKey, true)
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	if h.renderer == nil {
		h.writeJSON(w, http.StatusOK, map[string]any{"snapshot": snap, "error": errMsg})
		return
	}
	_ = h.renderer.RenderMarketplaceResults(w, r, snap, errMsg)
}

// HandleMarketplaceAddCatalog adds an item inspected in Marketplace directly into Catalog & Trap Guard.
func (h *Handlers) HandleMarketplaceAddCatalog(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Form tidak valid", http.StatusBadRequest)
		return
	}

	typeID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("type_id")), 10, 32)
	name := strings.TrimSpace(r.FormValue("name"))
	category := strings.TrimSpace(r.FormValue("category"))
	sellPrice, _ := strconv.ParseFloat(strings.TrimSpace(r.FormValue("sell_price")), 64)
	buyPrice, _ := strconv.ParseFloat(strings.TrimSpace(r.FormValue("buy_price")), 64)
	dailyVol, _ := strconv.ParseFloat(strings.TrimSpace(r.FormValue("daily_volume")), 64)

	item, err := h.svc.QuickAddMarketplaceItemToCatalog(r.Context(), int32(typeID), name, category, sellPrice, buyPrice, dailyVol)
	if err != nil {
		if IsHTMX(r) {
			TriggerToast(w, "error", "Failed adding to Catalog: "+err.Error())
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if IsHTMX(r) {
		TriggerToast(w, "success", "Item '"+item.Name+"' added to Catalog & Trap Guard.")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/catalog", http.StatusFound)
}
