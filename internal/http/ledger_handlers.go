package http

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"aura-forge/internal/domain"

	"github.com/go-chi/chi/v5"
)

// HandleLedgerSyncESI triggers Private ESI sync for wallet transactions, journal fees, active orders, and 90-day order history.
func (h *Handlers) HandleLedgerSyncESI(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.SyncPrivateESILedger(r.Context())
	if err != nil {
		if IsHTMX(r) {
			TriggerToast(w, "error", "Private ESI sync failed: "+err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	msg := fmt.Sprintf(
		"ESI Synced: %d new tx (%d dup), %d fee entries, %d active & %d historical orders.",
		res.TransactionsInserted, res.TransactionsSkipped, res.FeesInserted, res.ActiveOrdersSynced, res.OrderHistorySynced,
	)
	if IsHTMX(r) {
		TriggerToast(w, "success", msg)
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/tracker?tab=ledger", http.StatusSeeOther)
}

// HandleLedgerImportPaste imports copied rows from EVE Online Wallet -> Transactions.
func (h *Handlers) HandleLedgerImportPaste(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form payload", http.StatusBadRequest)
		return
	}
	raw := r.FormValue("paste_text")
	hubName := strings.TrimSpace(r.FormValue("hub_name"))

	ins, dup, bad, err := h.svc.ImportWalletTransactionsPaste(r.Context(), raw, hubName)
	if err != nil {
		if IsHTMX(r) {
			TriggerToast(w, "error", "Import failed: "+err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	msg := fmt.Sprintf("Imported %d new transaction(s) (%d duplicate skipped, %d unrecognized).", ins, dup, bad)
	if IsHTMX(r) {
		TriggerToast(w, "success", msg)
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/tracker?tab=ledger", http.StatusSeeOther)
}

// HandleLedgerAddManual saves a single manual BUY or SELL transaction.
func (h *Handlers) HandleLedgerAddManual(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form payload", http.StatusBadRequest)
		return
	}

	side := domain.TransactionSide(strings.ToUpper(strings.TrimSpace(r.FormValue("side"))))
	txDate := time.Now().UTC()
	if rawDate := strings.TrimSpace(r.FormValue("transacted_at")); rawDate != "" {
		if t, err := time.Parse("2006-01-02", rawDate); err == nil {
			txDate = t.UTC()
		}
	}

	tx := &domain.MarketTransaction{
		TransactedAt: txDate,
		Side:         side,
		ItemName:     strings.TrimSpace(r.FormValue("item_name")),
		Quantity:     parseIntForm(r, "quantity", 1),
		UnitPrice:    parseFloatForm(r, "unit_price", 0),
		EstimatedFee: parseFloatForm(r, "estimated_fee", 0),
		HubName:      strings.TrimSpace(r.FormValue("hub_name")),
		Notes:        strings.TrimSpace(r.FormValue("notes")),
	}

	if err := h.svc.AddManualTransaction(r.Context(), tx); err != nil {
		if IsHTMX(r) {
			TriggerToast(w, "error", "Failed to save transaction: "+err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if IsHTMX(r) {
		TriggerToast(w, "success", fmt.Sprintf("Recorded %s %dx %s.", tx.Side, tx.Quantity, tx.ItemName))
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/tracker?tab=ledger", http.StatusSeeOther)
}

// HandleLedgerDeleteTx deletes a single market transaction from the ledger.
func (h *Handlers) HandleLedgerDeleteTx(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "Invalid transaction ID", http.StatusBadRequest)
		return
	}
	if err := h.svc.DeleteLedgerTransaction(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if IsHTMX(r) {
		TriggerToast(w, "info", fmt.Sprintf("Transaction #%d deleted.", id))
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusOK)
}
