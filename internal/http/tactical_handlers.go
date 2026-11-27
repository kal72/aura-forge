package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"aura-forge/internal/domain"

	"github.com/go-chi/chi/v5"
)

// HandleTacticalPage renders the standalone fullscreen Aura Tactical Studio (/tactical).
func (h *Handlers) HandleTacticalPage(w http.ResponseWriter, r *http.Request) {
	data := h.svc.GetTacticalViewData(r.Context())
	if h.renderer != nil {
		if err := h.renderer.RenderTactical(w, r, data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	h.writeJSON(w, http.StatusOK, data)
}

// HandleTacticalShips returns local SDE ship hulls filtered by hull class & search query.
func (h *Handlers) HandleTacticalShips(w http.ResponseWriter, r *http.Request) {
	hullClass := r.URL.Query().Get("class")
	q := r.URL.Query().Get("q")
	items := h.svc.SearchTacticalShips(r.Context(), hullClass, q, 40)
	h.writeJSON(w, http.StatusOK, map[string]any{"ships": items})
}

// HandleTacticalModules returns local SDE modules/rigs/drones/charges for the active wheel slot.
func (h *Handlers) HandleTacticalModules(w http.ResponseWriter, r *http.Request) {
	slot := r.URL.Query().Get("slot")
	q := r.URL.Query().Get("q")
	items := h.svc.SearchTacticalModules(r.Context(), slot, q, 36)
	h.writeJSON(w, http.StatusOK, map[string]any{"modules": items})
}

// HandleTacticalSimulate runs deterministic EVE Dogma simulation for a ShipFit payload.
func (h *Handlers) HandleTacticalSimulate(w http.ResponseWriter, r *http.Request) {
	var fit domain.ShipFit
	if err := json.NewDecoder(r.Body).Decode(&fit); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid fitting JSON payload"})
		return
	}
	res, err := h.svc.SimulateTacticalFit(r.Context(), fit)
	if err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	h.writeJSON(w, http.StatusOK, res)
}

// HandleTacticalImportEFT parses an EFT clipboard dump, resolves all items concurrently, and simulates.
func (h *Handlers) HandleTacticalImportEFT(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EFT          string                    `json:"eft"`
		SkillProfile domain.SkillProfilePreset `json:"skill_profile"`
		TargetPreset string                    `json:"target_preset"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.EFT) == "" {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "please paste a valid EVE EFT fitting block"})
		return
	}
	res, err := h.svc.ImportTacticalEFT(r.Context(), req.EFT, req.SkillProfile, req.TargetPreset)
	if err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	h.writeJSON(w, http.StatusOK, res)
}

// HandleTacticalListFittings lists saved fittings for the current user.
func (h *Handlers) HandleTacticalListFittings(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.ListTacticalFittings(r.Context())
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]any{"fittings": list})
}

// HandleTacticalSaveFitting saves or updates a ShipFit in PostgreSQL.
func (h *Handlers) HandleTacticalSaveFitting(w http.ResponseWriter, r *http.Request) {
	var fit domain.ShipFit
	if err := json.NewDecoder(r.Body).Decode(&fit); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid fitting JSON payload"})
		return
	}
	rec, err := h.svc.SaveTacticalFitting(r.Context(), fit)
	if err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	h.writeJSON(w, http.StatusOK, rec)
}

// HandleTacticalDeleteFitting deletes a saved fitting by ID.
func (h *Handlers) HandleTacticalDeleteFitting(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid fitting id"})
		return
	}
	if err := h.svc.DeleteTacticalFitting(r.Context(), id); err != nil {
		h.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "id": id})
}

// HandleTacticalDuel runs a second-by-second 1v1 PvP/PvE boss combat timeline simulation.
func (h *Handlers) HandleTacticalDuel(w http.ResponseWriter, r *http.Request) {
	var req domain.DuelSimRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid duel simulation payload"})
		return
	}
	res, err := h.svc.SimulateTacticalDuel(r.Context(), req)
	if err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	h.writeJSON(w, http.StatusOK, res)
}

// HandleTacticalAIAudit runs Embedded Background AI (or deterministic fallback) structured audit on a ShipFit.
func (h *Handlers) HandleTacticalAIAudit(w http.ResponseWriter, r *http.Request) {
	var fit domain.ShipFit
	if err := json.NewDecoder(r.Body).Decode(&fit); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid fitting JSON payload"})
		return
	}
	res, err := h.svc.AuditTacticalFit(r.Context(), fit)
	if err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	h.writeJSON(w, http.StatusOK, res)
}

// HandleTacticalDScan classifies a live EVE Online Directional Scanner paste locally (0 ESI calls).
func (h *Handlers) HandleTacticalDScan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Raw string `json:"raw"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Raw) == "" {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "please paste a Directional Scanner dump from EVE Online"})
		return
	}
	res, err := domain.AnalyzeDScan(req.Raw)
	if err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	h.writeJSON(w, http.StatusOK, res)
}

// HandleTacticalCombatLog parses an EVE Online Gamelog combat paste deterministically (0 AI tokens).
func (h *Handlers) HandleTacticalCombatLog(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Raw string `json:"raw"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Raw) == "" {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "please paste lines from EVE Online Gamelogs/*.txt"})
		return
	}
	res, err := domain.AnalyzeCombatLog(req.Raw)
	if err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	h.writeJSON(w, http.StatusOK, res)
}

// HandleTacticalFlyToday downgrades Tech 2 modules into beginner-accessible Compact/Meta/T1 equivalents.
func (h *Handlers) HandleTacticalFlyToday(w http.ResponseWriter, r *http.Request) {
	var fit domain.ShipFit
	if err := json.NewDecoder(r.Body).Decode(&fit); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid fitting JSON payload"})
		return
	}
	res, err := h.svc.AdaptFitToFlyToday(r.Context(), fit)
	if err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	h.writeJSON(w, http.StatusOK, res)
}

// HandleEmbeddedDailyStrip hydrates the non-blocking Dashboard Daily Action Strip via HTMX.
func (h *Handlers) HandleEmbeddedDailyStrip(w http.ResponseWriter, r *http.Request) {
	strip := h.svc.GetDashboardDailyStrip(r.Context())
	if h.renderer != nil {
		if err := h.renderer.RenderDashboardDailyStrip(w, r, strip); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	h.writeJSON(w, http.StatusOK, strip)
}

// HandleTacticalCompareFits simulates two fittings concurrently for side-by-side comparison.
func (h *Handlers) HandleTacticalCompareFits(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FitA domain.ShipFit `json:"fit_a"`
		FitB domain.ShipFit `json:"fit_b"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid compare payload"})
		return
	}
	res, err := h.svc.CompareTacticalFits(r.Context(), req.FitA, req.FitB)
	if err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	h.writeJSON(w, http.StatusOK, res)
}
