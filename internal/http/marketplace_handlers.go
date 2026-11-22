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
		hubKey = h.svc.ActiveCatalogHub(r.Context()).Key
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
		hubKey = h.svc.ActiveCatalogHub(r.Context()).Key
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

// HandleMarketplaceAutocomplete returns fast local SDE type matches (0 ESI calls) for the search box.
func (h *Handlers) HandleMarketplaceAutocomplete(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	items := h.svc.SearchMarketplaceTypes(r.Context(), q, 12)
	h.writeJSON(w, http.StatusOK, items)
}

// HandleMarketplaceBrowse returns paginated EVE market items grouped by category & sub-group from local PostgreSQL sde_types (0 ESI calls).
func (h *Handlers) HandleMarketplaceBrowse(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	category := strings.TrimSpace(r.URL.Query().Get("category"))
	groupName := strings.TrimSpace(r.URL.Query().Get("group"))
	page := parseIntDefault(r.URL.Query().Get("page"), 1)
	limit := parseIntDefault(r.URL.Query().Get("limit"), 24)

	res, err := h.svc.BrowseMarketplaceTypes(r.Context(), q, category, groupName, page, limit)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	h.writeJSON(w, http.StatusOK, res)
}

// HandleMarketplaceAppraise evaluates pasted EVE Online clipboard items (Inventory, Cargo, Multibuy, Fit)
// against live prices at the chosen Trade Hub.
func (h *Handlers) HandleMarketplaceAppraise(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form payload", http.StatusBadRequest)
		return
	}
	rawText := strings.TrimSpace(r.FormValue("paste_text"))
	hubKey := strings.TrimSpace(r.FormValue("hub"))
	buybackPct := parseFloatDefault(r.FormValue("buyback_pct"), 90.0)

	res, err := h.svc.AppraiseEVEClipboard(r.Context(), rawText, hubKey, buybackPct)
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	if h.renderer == nil {
		h.writeJSON(w, http.StatusOK, map[string]any{"appraisal": res, "error": errMsg})
		return
	}
	_ = h.renderer.RenderMarketplaceAppraisal(w, r, res, errMsg)
}

// HandleMarketplaceAddCatalog adds an item inspected in Marketplace directly into Catalog & Trap Guard.
func (h *Handlers) HandleMarketplaceAddCatalog(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form payload", http.StatusBadRequest)
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
