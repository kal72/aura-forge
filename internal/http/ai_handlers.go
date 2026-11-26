package http

import (
	"html"
	"net/http"
	"strconv"
	"strings"

	"aura-forge/internal/domain"
	"aura-forge/internal/service"

	"github.com/go-chi/chi/v5"
)

// HandleAIDrawer loads the Aura AI Copilot drawer state (recent conversations & active thread messages).
func (h *Handlers) HandleAIDrawer(w http.ResponseWriter, r *http.Request) {
	mode := h.svc.GetModeInfo(r.Context())
	convID, _ := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("conv_id")), 10, 64)
	convs, _ := h.svc.ListAIConversations(r.Context())

	var msgs []domain.AIMessage
	if convID > 0 {
		exists := false
		for _, c := range convs {
			if c.ID == convID {
				exists = true
				break
			}
		}
		if exists {
			msgs, _ = h.svc.GetAIConversationMessages(r.Context(), convID)
		} else {
			convID = 0
		}
	}

	if h.renderer == nil {
		h.writeJSON(w, http.StatusOK, map[string]any{
			"ai_enabled":    mode.AI.Enabled,
			"conversations": convs,
			"messages":      msgs,
		})
		return
	}
	_ = h.renderer.RenderAIDrawerThread(w, r, mode, convs, convID, msgs, "")
}

// HandleAIChat executes one Level-4 Agentic Copilot turn and returns the updated conversation thread partial.
func (h *Handlers) HandleAIChat(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}
	mode := h.svc.GetModeInfo(r.Context())
	convID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("conversation_id")), 10, 64)
	prompt := strings.TrimSpace(r.FormValue("prompt"))
	ctxPage := strings.TrimSpace(r.FormValue("context_page"))
	ctxSummary := strings.TrimSpace(r.FormValue("context_summary"))

	res, err := h.svc.AskAuraCopilot(r.Context(), service.AIChatTurnInput{
		ConversationID: convID,
		Prompt:         prompt,
		ContextPage:    ctxPage,
		ContextSummary: ctxSummary,
	})
	convs, _ := h.svc.ListAIConversations(r.Context())

	if err != nil {
		var existing []domain.AIMessage
		if convID > 0 {
			existing, _ = h.svc.GetAIConversationMessages(r.Context(), convID)
		}
		if h.renderer == nil {
			h.writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		_ = h.renderer.RenderAIDrawerThread(w, r, mode, convs, convID, existing, err.Error())
		return
	}

	if h.renderer == nil {
		h.writeJSON(w, http.StatusOK, res)
		return
	}
	_ = h.renderer.RenderAIDrawerThread(w, r, mode, convs, res.ConversationID, res.Messages, "")
}

// HandleAIDeleteConversation deletes one saved Aura AI conversation owned by the current user.
func (h *Handlers) HandleAIDeleteConversation(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err == nil && id > 0 {
		_ = h.svc.DeleteAIConversation(r.Context(), id)
	}
	mode := h.svc.GetModeInfo(r.Context())
	convs, _ := h.svc.ListAIConversations(r.Context())
	if h.renderer == nil {
		h.writeJSON(w, http.StatusOK, map[string]any{"deleted": id})
		return
	}
	_ = h.renderer.RenderAIDrawerThread(w, r, mode, convs, 0, nil, "")
}

// HandleAITestConnection tests connectivity to the configured OpenAI-compatible AI provider.
func (h *Handlers) HandleAITestConnection(w http.ResponseWriter, r *http.Request) {
	msg, err := h.svc.TestAIConnection(r.Context())
	if err != nil {
		cleanErr := strings.Join(strings.Fields(err.Error()), " ")
		if IsHTMX(r) {
			TriggerToast(w, "error", "AI connection failed: "+cleanErr)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<div class="rounded-lg border border-rose-400/45 bg-rose-950/40 px-3 py-2 text-xs font-mono text-rose-200 break-words"><strong>✕ Connection Failed:</strong> ` + html.EscapeString(cleanErr) + `</div>`))
			return
		}
		h.writeJSON(w, http.StatusBadGateway, map[string]string{"error": cleanErr})
		return
	}
	if IsHTMX(r) {
		TriggerToast(w, "success", msg)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<div class="rounded-lg border border-emerald-400/45 bg-emerald-950/40 px-3 py-2 text-xs font-mono text-emerald-200 break-words"><strong>✓ Connected:</strong> ` + html.EscapeString(msg) + `</div>`))
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]string{"status": msg})
}

// HandleAISetModel overrides the active AI model in memory (not persisted to PostgreSQL).
func (h *Handlers) HandleAISetModel(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}
	model := strings.TrimSpace(r.FormValue("ai_model"))
	updated, err := h.svc.SetAIModel(model)
	if err != nil {
		cleanErr := strings.Join(strings.Fields(err.Error()), " ")
		if IsHTMX(r) {
			TriggerToast(w, "error", "Failed to update AI model: "+cleanErr)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": cleanErr})
		return
	}
	if IsHTMX(r) {
		TriggerToast(w, "success", "Active AI model updated in memory: "+updated)
		w.Header().Set("HX-Redirect", "/settings?saved=1")
		w.WriteHeader(http.StatusOK)
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]string{"model": updated})
}

// HandleAISaveMemory persists the user's Pilot Playbook Memory (playstyle, risk tolerance, daily playtime).
func (h *Handlers) HandleAISaveMemory(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}
	notes := r.FormValue("playstyle_notes")
	risk := r.FormValue("risk_tolerance")
	playtime := r.FormValue("daily_playtime")

	if _, err := h.svc.SavePilotMemory(r.Context(), notes, risk, playtime); err != nil {
		if IsHTMX(r) {
			TriggerToast(w, "error", "Failed to save AI Playbook Memory: "+err.Error())
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if IsHTMX(r) {
		TriggerToast(w, "success", "Aura AI Pilot Playbook Memory saved.")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/settings?saved=1", http.StatusSeeOther)
}
