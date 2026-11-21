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

	// Optionally pre-load a recipe tree if ?type_id= is provided (default: Robotics 9848 without blocking on ESI)
	var selectedTree *domain.PIRecipeAnalysis
	if rawID := strings.TrimSpace(r.URL.Query().Get("type_id")); rawID != "" {
		if id, err := strconv.ParseInt(rawID, 10, 32); err == nil && id > 0 {
			pocoTax := parseFloatDefault(r.URL.Query().Get("poco_tax"), 5.0)
			if res, err := h.svc.InspectPIRecipeTree(r.Context(), int32(id), pocoTax); err == nil {
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

// HandlePITree returns the HTMX partial with the full recursive PI Recipe Tree, Source Planets, and live Jita economics.
func (h *Handlers) HandlePITree(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("type_id")), 10, 32)
	if err != nil || id <= 0 {
		id, _ = strconv.ParseInt(strings.TrimSpace(r.FormValue("type_id")), 10, 32)
	}
	pocoTax := parseFloatDefault(r.URL.Query().Get("poco_tax"), parseFloatDefault(r.FormValue("poco_tax"), 5.0))

	res, rErr := h.svc.InspectPIRecipeTree(r.Context(), int32(id), pocoTax)
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

// HandlePIScan scans all PI items in a selected tier (P1, P2, P3, or P4) against live Jita Public ESI prices.
func (h *Handlers) HandlePIScan(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}
	tier := strings.TrimSpace(r.FormValue("tier"))
	pocoTax := parseFloatDefault(r.FormValue("poco_tax"), 5.0)

	rows, err := h.svc.ScanPIProfitability(r.Context(), tier, pocoTax)
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	if h.renderer == nil {
		h.writeJSON(w, http.StatusOK, map[string]any{"rows": rows, "error": errMsg})
		return
	}
	_ = h.renderer.RenderPIScanPartial(w, r, rows, tier, errMsg)
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
