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
