package http

import "net/http"

// HandleWikiPage renders the built-in EVE Online Glossary, UI Color Legend, and Beginner Guide (/wiki).
func (h *Handlers) HandleWikiPage(w http.ResponseWriter, r *http.Request) {
	if h.renderer == nil {
		h.writeJSON(w, http.StatusOK, map[string]string{"status": "wiki_ready"})
		return
	}
	mode := h.svc.GetModeInfo(r.Context())
	_ = h.renderer.RenderWiki(w, r, mode)
}
