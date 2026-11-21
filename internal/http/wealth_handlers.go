package http

import (
	"net/http"
	"strconv"
	"strings"

	"aura-forge/internal/domain"

	"github.com/go-chi/chi/v5"
)

// HandleWealthPage renders the Net Worth Tracker (0 -> 100B+ Milestone Curve) & Beginner Capital-Phase Advisor.
func (h *Handlers) HandleWealthPage(w http.ResponseWriter, r *http.Request) {
	summary, err := h.svc.GetWealthSummary(r.Context())
	if err != nil {
		http.Error(w, "Failed loading wealth summary: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if h.renderer == nil {
		h.writeJSON(w, http.StatusOK, summary)
		return
	}
	mode := h.svc.GetModeInfo(r.Context())
	_ = h.renderer.RenderWealth(w, r, mode, summary)
}

// HandleSaveWealthSnapshot records or updates a daily Net Worth snapshot.
func (h *Handlers) HandleSaveWealthSnapshot(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	snap := &domain.NetWorthSnapshot{
		SnapshotDate:  strings.TrimSpace(r.FormValue("snapshot_date")),
		WalletISK:     parseFloatDefault(r.FormValue("wallet_isk"), 0),
		SellOrdersISK: parseFloatDefault(r.FormValue("sell_orders_isk"), 0),
		EscrowISK:     parseFloatDefault(r.FormValue("escrow_isk"), 0),
		InventoryISK:  parseFloatDefault(r.FormValue("inventory_isk"), 0),
		ActiveJobsISK: parseFloatDefault(r.FormValue("active_jobs_isk"), 0),
		Note:          strings.TrimSpace(r.FormValue("note")),
	}

	saved, err := h.svc.RecordNetWorthSnapshot(r.Context(), snap)
	if err != nil {
		if IsHTMX(r) {
			TriggerToast(w, "error", "Failed saving snapshot: "+err.Error())
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if IsHTMX(r) {
		TriggerToast(w, "success", "Net Worth snapshot saved for "+saved.SnapshotDate+".")
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/wealth", http.StatusFound)
}

// HandleAutoWealthSnapshot captures today's Net Worth snapshot automatically from live Wallet & Tracker batches.
func (h *Handlers) HandleAutoWealthSnapshot(w http.ResponseWriter, r *http.Request) {
	saved, err := h.svc.CaptureAutoNetWorthSnapshot(r.Context())
	if err != nil {
		if IsHTMX(r) {
			TriggerToast(w, "error", "Auto-snapshot failed: "+err.Error())
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if IsHTMX(r) {
		TriggerToast(w, "success", "Auto-snapshot captured for "+saved.SnapshotDate+".")
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/wealth", http.StatusFound)
}

// HandleDeleteWealthSnapshot removes a recorded Net Worth snapshot.
func (h *Handlers) HandleDeleteWealthSnapshot(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid snapshot ID", http.StatusBadRequest)
		return
	}
	if err := h.svc.DeleteNetWorthSnapshot(r.Context(), id); err != nil {
		if IsHTMX(r) {
			TriggerToast(w, "error", "Failed deleting snapshot: "+err.Error())
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if IsHTMX(r) {
		TriggerToast(w, "info", "Snapshot deleted.")
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/wealth", http.StatusFound)
}
