package http

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"aura-forge/internal/domain"

	"github.com/go-chi/chi/v5"
)

// HandlePIPage renders the Planetary Industry (PI) Hub: P1–P4 Explorer, Recipe Tree + Source Planets, Profit Scanner, and Colony Timers.
func (h *Handlers) HandlePIPage(w http.ResponseWriter, r *http.Request) {
	mode := h.svc.GetModeInfo(r.Context())
	items := domain.ListProduciblePICommodities("ALL", "")
	colonies, _ := h.svc.ListPIColonies(r.Context())

	// Optionally pre-load a recipe tree if ?type_id= is provided
	var selectedTree *domain.PIRecipeAnalysis
	if rawID := strings.TrimSpace(r.URL.Query().Get("type_id")); rawID != "" {
		if id, err := strconv.ParseInt(rawID, 10, 32); err == nil && id > 0 {
			pocoTax := parseFloatDefault(r.URL.Query().Get("poco_tax"), 5.0)
			hubKey := strings.TrimSpace(r.URL.Query().Get("hub"))
			if res, err := h.svc.InspectPIRecipeTree(r.Context(), int32(id), pocoTax, hubKey); err == nil {
				selectedTree = res
			}
		}
	}

	if h.renderer == nil {
		h.writeJSON(w, http.StatusOK, map[string]any{"items": len(items), "colonies": len(colonies)})
		return
	}
	_ = h.renderer.RenderPI(w, r, mode, items, selectedTree, colonies)
}

// HandlePITree returns the HTMX partial with the full recursive PI Recipe Tree, Source Planets, and live hub economics.
func (h *Handlers) HandlePITree(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("type_id")), 10, 32)
	if err != nil || id <= 0 {
		id, _ = strconv.ParseInt(strings.TrimSpace(r.FormValue("type_id")), 10, 32)
	}
	pocoTax := parseFloatDefault(r.URL.Query().Get("poco_tax"), parseFloatDefault(r.FormValue("poco_tax"), 5.0))
	hubKey := strings.TrimSpace(r.URL.Query().Get("hub"))
	if hubKey == "" {
		hubKey = strings.TrimSpace(r.FormValue("hub"))
	}

	res, rErr := h.svc.InspectPIRecipeTree(r.Context(), int32(id), pocoTax, hubKey)
	errMsg := ""
	if rErr != nil {
		errMsg = rErr.Error()
	}

	if h.renderer == nil {
		h.writeJSON(w, http.StatusOK, map[string]any{"tree": res, "error": errMsg})
		return
	}
	_ = h.renderer.RenderPITreePartial(w, r, res, errMsg)
}

// HandlePIScan scans all PI items in a selected tier (P1, P2, P3, or P4) against live Public ESI prices at the chosen hub.
func (h *Handlers) HandlePIScan(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}
	tier := strings.TrimSpace(r.FormValue("tier"))
	pocoTax := parseFloatDefault(r.FormValue("poco_tax"), 5.0)
	hubKey := strings.TrimSpace(r.FormValue("hub"))

	rows, hub, err := h.svc.ScanPIProfitability(r.Context(), tier, pocoTax, hubKey)
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	if h.renderer == nil {
		h.writeJSON(w, http.StatusOK, map[string]any{"rows": rows, "hub": hub, "error": errMsg})
		return
	}
	_ = h.renderer.RenderPIScanPartial(w, r, rows, tier, hub, errMsg)
}

// HandlePISync syncs live Planetary Industry colonies (planets, locations, P0–P4 production, storage, expiry) from EVE SSO.
func (h *Handlers) HandlePISync(w http.ResponseWriter, r *http.Request) {
	count, err := h.svc.SyncPIColoniesFromSSO(r.Context())
	if err != nil {
		if IsHTMX(r) {
			TriggerToast(w, "error", "PI Sync failed: "+err.Error())
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if IsHTMX(r) {
		TriggerToast(w, "success", "Synced "+strconv.Itoa(count)+" live PI planet colonies from EVE Online SSO.")
		w.Header().Set("HX-Redirect", "/pi?tab=colonies")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/pi?tab=colonies", http.StatusFound)
}

// HandlePIPlanetLayout returns JSON pin coordinates, extractor heads, links, and routes for the HTML5 canvas visualizer.
func (h *Handlers) HandlePIPlanetLayout(w http.ResponseWriter, r *http.Request) {
	rawID := strings.TrimSpace(chi.URLParam(r, "planet_id"))
	pid, err := strconv.ParseInt(rawID, 10, 32)
	if err != nil || pid <= 0 {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid planet_id"})
		return
	}
	layout, err := h.svc.GetPIPlanetLayout(r.Context(), int32(pid))
	if err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	h.writeJSON(w, http.StatusOK, layout)
}

// HandlePISalesCalc simulates multi-item PI sales profit for custom unit quantities (from row builder or EVE Hangar paste).
func (h *Handlers) HandlePISalesCalc(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	hubKey := strings.TrimSpace(r.FormValue("hub"))
	costMode := strings.TrimSpace(r.FormValue("cost_mode"))
	sellMode := strings.TrimSpace(r.FormValue("sell_mode"))
	pocoTax := parseFloatDefault(r.FormValue("poco_tax"), 5.0)
	pasteText := strings.TrimSpace(r.FormValue("paste_text"))

	var items []domain.PISalesInputItem
	var unmatched []string

	if pasteText != "" {
		parsed, bad := domain.ParsePIInventoryPaste(pasteText)
		items = append(items, parsed...)
		unmatched = bad
	}

	rawIDs := r.Form["type_id"]
	rawQtys := r.Form["quantity"]
	for i := 0; i < len(rawIDs); i++ {
		tid, err := strconv.ParseInt(strings.TrimSpace(rawIDs[i]), 10, 32)
		if err != nil || tid <= 0 {
			continue
		}
		var qty int64
		if i < len(rawQtys) {
			qty, _ = strconv.ParseInt(strings.TrimSpace(rawQtys[i]), 10, 64)
		}
		if qty <= 0 {
			continue
		}
		items = append(items, domain.PISalesInputItem{
			TypeID:   int32(tid),
			Quantity: qty,
		})
	}

	res, err := h.svc.SimulatePISales(r.Context(), items, costMode, sellMode, pocoTax, hubKey)
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	if h.renderer == nil {
		h.writeJSON(w, http.StatusOK, map[string]any{"result": res, "unmatched": unmatched, "error": errMsg})
		return
	}
	_ = h.renderer.RenderPISalesCalcPartial(w, r, res, unmatched, errMsg)
}

// HandlePIColonySave creates a new tracked PI colony with an extractor expiry timer.
func (h *Handlers) HandlePIColonySave(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	cycleHours := parseIntDefault(r.FormValue("cycle_hours"), 48)
	if cycleHours <= 0 {
		cycleHours = 48
	}

	prodID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("product_type_id")), 10, 32)
	prodName := strings.TrimSpace(r.FormValue("product_name"))
	tier := domain.PITier(strings.ToUpper(strings.TrimSpace(r.FormValue("tier"))))
	if prodID > 0 {
		if comm, ok := domain.GetPICommodity(int32(prodID)); ok {
			prodName = comm.Name
			tier = comm.Tier
		}
	}
	if tier == "" {
		tier = domain.PITierP2
	}

	colony := &domain.PIColony{
		PlanetName:         strings.TrimSpace(r.FormValue("planet_name")),
		SolarSystem:        strings.TrimSpace(r.FormValue("solar_system")),
		PlanetType:         strings.TrimSpace(r.FormValue("planet_type")),
		ProductTypeID:      int32(prodID),
		ProductName:        prodName,
		Tier:               tier,
		ExtractorExpiresAt: time.Now().Add(time.Duration(cycleHours) * time.Hour),
		EstDailyISK:        parseFloatDefault(r.FormValue("est_daily_isk"), 0),
		Note:               strings.TrimSpace(r.FormValue("note")),
	}

	saved, err := h.svc.SavePIColony(r.Context(), colony)
	if err != nil {
		if IsHTMX(r) {
			TriggerToast(w, "error", "Failed saving PI colony: "+err.Error())
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if IsHTMX(r) {
		TriggerToast(w, "success", "PI colony '"+saved.PlanetName+"' added.")
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/pi", http.StatusFound)
}

// HandlePIColonyReset resets a colony's extractor timer by N hours (e.g. 48h or 72h).
func (h *Handlers) HandlePIColonyReset(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid colony ID", http.StatusBadRequest)
		return
	}
	hours := parseIntDefault(r.URL.Query().Get("hours"), parseIntDefault(r.FormValue("hours"), 48))
	if err := h.svc.ResetPIColonyExtractor(r.Context(), id, hours); err != nil {
		if IsHTMX(r) {
			TriggerToast(w, "error", "Failed resetting extractor: "+err.Error())
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if IsHTMX(r) {
		TriggerToast(w, "success", "Extractor timer reset for +"+strconv.Itoa(hours)+" hours.")
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/pi", http.StatusFound)
}

// HandlePIColonyDelete deletes a tracked PI colony.
func (h *Handlers) HandlePIColonyDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid colony ID", http.StatusBadRequest)
		return
	}
	if err := h.svc.DeletePIColony(r.Context(), id); err != nil {
		if IsHTMX(r) {
			TriggerToast(w, "error", "Failed deleting colony: "+err.Error())
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if IsHTMX(r) {
		TriggerToast(w, "info", "PI colony removed.")
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/pi", http.StatusFound)
}
