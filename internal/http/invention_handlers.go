package http

import (
	"net/http"
	"strings"

	"aura-forge/internal/domain"
)

// HandleInventionPage renders the T2 Invention & Decryptor Optimizer + Make-vs-Buy Component Analyzer page.
func (h *Handlers) HandleInventionPage(w http.ResponseWriter, r *http.Request) {
	mode := h.svc.GetModeInfo(r.Context())
	settings, _ := h.svc.GetSettings(r.Context())
	if h.renderer == nil {
		h.writeJSON(w, http.StatusOK, map[string]any{"status": "invention_ready"})
		return
	}
	_ = h.renderer.RenderInvention(w, r, mode, *settings)
}

// HandleInventionOptimize runs the live 9-Decryptor T2 Invention optimizer via HTMX.
func (h *Handlers) HandleInventionOptimize(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	settings, _ := h.svc.GetSettings(r.Context())
	in := domain.InventionInput{
		PresetKey:           strings.TrimSpace(r.FormValue("preset_key")),
		HubKey:              strings.TrimSpace(r.FormValue("hub")),
		EncryptionSkill:     parseIntDefault(r.FormValue("encryption_skill"), 4),
		Datacore1Skill:      parseIntDefault(r.FormValue("datacore_1_skill"), 4),
		Datacore2Skill:      parseIntDefault(r.FormValue("datacore_2_skill"), 4),
		BaseBuildCostPerRun: parseFloatDefault(r.FormValue("base_build_cost_per_run"), 0),
		ProductSellOverride: parseFloatDefault(r.FormValue("product_sell_override"), 0),
		InventionJobFee:     parseFloatDefault(r.FormValue("invention_job_fee"), 50_000),
		BrokerFeePercent:    settings.BrokerFeePercent,
		SalesTaxPercent:     settings.SalesTaxPercent,
	}

	res, err := h.svc.OptimizeT2InventionLive(r.Context(), in)
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	if h.renderer == nil {
		h.writeJSON(w, http.StatusOK, map[string]any{"result": res, "error": errMsg})
		return
	}
	_ = h.renderer.RenderInventionResult(w, r, res, errMsg)
}

// HandleMakeVsBuy evaluates whether an intermediate component is cheaper to manufacture or buy in Jita.
func (h *Handlers) HandleMakeVsBuy(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("component_name"))
	qty := parseIntDefault(r.FormValue("quantity"), 100)
	jitaPrice := parseFloatDefault(r.FormValue("jita_unit_price"), 0)
	rawCost := parseFloatDefault(r.FormValue("raw_material_per_unit"), 0)

	res, err := h.svc.EvaluateMakeVsBuy(r.Context(), name, qty, jitaPrice, rawCost)
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	if h.renderer == nil {
		h.writeJSON(w, http.StatusOK, map[string]any{"result": res, "error": errMsg})
		return
	}
	_ = h.renderer.RenderMakeVsBuyResult(w, r, &res, errMsg)
}
