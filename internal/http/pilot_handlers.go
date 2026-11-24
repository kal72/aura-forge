package http

import (
	"fmt"
	"net/http"
	"strconv"
)

// HandlePilotHub renders the dedicated EVE Online Pilot Hub (/pilot) showing live SSO character
// Wallet, Available Capital, Active Jobs, Open Orders, Blueprints, and Hangar Stock with search, filter & pagination.
func (h *Handlers) HandlePilotHub(w http.ResponseWriter, r *http.Request) {
	activeTab := r.URL.Query().Get("tab")
	searchQuery := r.URL.Query().Get("q")
	typeFilter := r.URL.Query().Get("filter")
	page := 1
	if rawPage := r.URL.Query().Get("page"); rawPage != "" {
		if p, err := strconv.Atoi(rawPage); err == nil && p > 0 {
			page = p
		}
	}

	view, err := h.svc.GetPilotHubView(r.Context(), activeTab, searchQuery, typeFilter, page)
	if err != nil {
		http.Error(w, "Failed to load EVE Pilot Hub: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if h.renderer != nil {
		mode := h.svc.GetModeInfo(r.Context())
		_ = h.renderer.RenderPilotHub(w, r, mode, view)
		return
	}
	h.writeJSON(w, http.StatusOK, view)
}

// HandlePilotSyncCapital explicitly copies Live EVE Wallet Balance into StartingCapital on user click.
func (h *Handlers) HandlePilotSyncCapital(w http.ResponseWriter, r *http.Request) {
	newCap, err := h.svc.SyncWorkingCapitalFromWallet(r.Context())
	if err != nil {
		if IsHTMX(r) {
			TriggerToast(w, "error", "Could not sync capital: "+err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if IsHTMX(r) {
		TriggerToast(w, "success", fmt.Sprintf("Capital updated to %.2f ISK from EVE Wallet.", newCap))
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/pilot", http.StatusSeeOther)
}
