package http

import (
	"net/http"
	"strconv"
	"strings"

	"aura-forge/internal/domain"
)

// HandleBlueprintCompute calculates an SDE Blueprint BOM, live Jita material/product prices, build time, ISK/hour, and profit.
func (h *Handlers) HandleBlueprintCompute(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	settings, _ := h.svc.GetSettings(r.Context())
	bpID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("blueprint_type_id")), 10, 32)

	in := domain.BlueprintCalcInput{
		BlueprintTypeID:     int32(bpID),
		Runs:                parseIntDefault(r.FormValue("runs"), 1),
		ME:                  parseIntDefault(r.FormValue("me"), 10),
		TE:                  parseIntDefault(r.FormValue("te"), 20),
		StructureKey:        strings.TrimSpace(r.FormValue("structure_key")),
		HubKey:              strings.TrimSpace(r.FormValue("hub_key")),
		MaterialPriceSource: strings.TrimSpace(r.FormValue("material_price_source")),
		MakeBuyOverrides:    domain.ParseMakeBuyOverrides(r.FormValue("make_buy_overrides")),
		ProductSellOverride: parseFloatDefault(r.FormValue("product_sell_override"), 0),
		ExtraSetupCost:      parseFloatDefault(r.FormValue("extra_setup_cost"), 0),
		SystemCostIndex:     parseFloatDefault(r.FormValue("system_cost_index"), settings.SystemCostIndex),
		FacilityTaxPercent:  parseFloatDefault(r.FormValue("facility_tax_percent"), settings.FacilityTaxPercent),
		FreightCostPerM3:    parseFloatDefault(r.FormValue("freight_cost_per_m3"), settings.FreightCostPerM3),
		BrokerFeePercent:    parseFloatDefault(r.FormValue("broker_fee_percent"), settings.BrokerFeePercent),
		SalesTaxPercent:     parseFloatDefault(r.FormValue("sales_tax_percent"), settings.SalesTaxPercent),
		RelistFeePercent:    settings.RelistFeePercent,
		EstimatedRelists:    parseIntDefault(r.FormValue("estimated_relists"), 1),
		BatchesPerWeek:      parseIntDefault(r.FormValue("batches_per_week"), 4),
	}

	res, err := h.svc.CalculateBlueprintProfit(r.Context(), in)
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	if h.renderer != nil {
		_ = h.renderer.RenderBlueprintResult(w, r, res, errMsg)
		return
	}
	if err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": errMsg})
		return
	}
	h.writeJSON(w, http.StatusOK, res)
}

// HandleBlueprintSaveCatalog saves an SDE blueprint calculation directly into Catalog & Trap Guard.
func (h *Handlers) HandleBlueprintSaveCatalog(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	typeID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("product_type_id")), 10, 32)
	name := strings.TrimSpace(r.FormValue("product_name"))
	category := strings.TrimSpace(r.FormValue("category"))
	if category == "" {
		category = "Other"
	}

	item := &domain.MarketItem{
		TypeID:            int32(typeID),
		Name:              name,
		Category:          domain.ItemCategory(category),
		BatchOutputQty:    parseIntDefault(r.FormValue("output_qty"), 1),
		EstimatedJitaSell: parseFloatDefault(r.FormValue("unit_sell_price"), 0),
		EstimatedJitaBuy:  parseFloatDefault(r.FormValue("unit_buy_price"), 0),
		MaterialCost:      parseFloatDefault(r.FormValue("material_cost"), 0),
		ExtraSetupCost:    parseFloatDefault(r.FormValue("extra_setup_cost"), 0),
		DailyVolume:       100,
		IsManufacturable:  true,
	}

	saved, err := h.svc.SaveCatalogItem(r.Context(), item)
	if err != nil {
		if IsHTMX(r) {
			TriggerToast(w, "error", "Failed to save to Catalog: "+err.Error())
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if IsHTMX(r) {
		TriggerToast(w, "success", "Blueprint output '"+saved.Name+"' saved to Catalog & Trap Guard.")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/catalog", http.StatusFound)
}

// HandleBlueprintSearch returns paginated SDE blueprints filtered by query, category, and tech level
// for the Blueprint Catalog Modal Popup in /calculator.
func (h *Handlers) HandleBlueprintSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	category := strings.TrimSpace(r.URL.Query().Get("category"))
	tech := parseIntDefault(r.URL.Query().Get("tech"), 0)
	page := parseIntDefault(r.URL.Query().Get("page"), 1)
	pageSize := parseIntDefault(r.URL.Query().Get("limit"), 24)

	res, err := h.svc.SearchBlueprintsPaged(r.Context(), q, category, tech, page, pageSize)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	h.writeJSON(w, http.StatusOK, res)
}

// HandleManufacturePage renders the dedicated Manufacture Horizontal Blueprint Tree & Recipe Cost page.
func (h *Handlers) HandleManufacturePage(w http.ResponseWriter, r *http.Request) {
	mode := h.svc.GetModeInfo(r.Context())
	settings, _ := h.svc.GetSettings(r.Context())
	blueprints, _ := h.svc.SearchBlueprints(r.Context(), "", "", 100)

	var initialRes *domain.BlueprintCalcResult
	rawBP := strings.TrimSpace(r.URL.Query().Get("bp_id"))
	if rawBP == "" {
		rawBP = strings.TrimSpace(r.URL.Query().Get("bp"))
	}
	if rawBP != "" {
		if bpID, err := strconv.ParseInt(rawBP, 10, 32); err == nil && bpID > 0 {
			hubKey := strings.TrimSpace(r.URL.Query().Get("hub"))
			if hubKey == "" && settings != nil {
				hubKey = settings.ActiveTradeHub().Key
			}
			me := 10
			if rawME := strings.TrimSpace(r.URL.Query().Get("me")); rawME != "" {
				if v, err := strconv.Atoi(rawME); err == nil && v >= 0 && v <= 10 {
					me = v
				}
			}
			te := 20
			if rawTE := strings.TrimSpace(r.URL.Query().Get("te")); rawTE != "" {
				if v, err := strconv.Atoi(rawTE); err == nil && v >= 0 && v <= 20 {
					te = v
				}
			}
			in := domain.BlueprintCalcInput{
				BlueprintTypeID:     int32(bpID),
				Runs:                parseIntDefault(r.URL.Query().Get("runs"), 1),
				ME:                  me,
				TE:                  te,
				HubKey:              hubKey,
				MaterialPriceSource: "jita_sell",
				SystemCostIndex:     settings.SystemCostIndex,
				FacilityTaxPercent:  settings.FacilityTaxPercent,
				BrokerFeePercent:    settings.BrokerFeePercent,
				SalesTaxPercent:     settings.SalesTaxPercent,
				RelistFeePercent:    settings.RelistFeePercent,
				EstimatedRelists:    1,
				BatchesPerWeek:      1,
			}
			if res, err := h.svc.CalculateBlueprintProfit(r.Context(), in); err == nil {
				initialRes = res
			}
		}
	}

	if h.renderer != nil {
		_ = h.renderer.RenderManufacture(w, r, mode, *settings, blueprints, initialRes)
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]any{"blueprints": len(blueprints), "result": initialRes})
}

// HandleManufactureTree computes the horizontal blueprint recipe tree and total recipe cost for the chosen Trade Hub.
func (h *Handlers) HandleManufactureTree(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	settings, _ := h.svc.GetSettings(r.Context())
	bpID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("blueprint_type_id")), 10, 32)
	hubKey := strings.TrimSpace(r.FormValue("hub_key"))
	if hubKey == "" && settings != nil {
		hubKey = settings.ActiveTradeHub().Key
	}

	// Smart Auto defaults when Advanced Override fields are left blank (0ms deterministic)
	sci := settings.SystemCostIndex
	if raw := strings.TrimSpace(r.FormValue("system_cost_index")); raw != "" {
		if v, err := strconv.ParseFloat(raw, 64); err == nil && v >= 0 {
			sci = v
		}
	}
	freightPerM3 := 0.0
	if raw := strings.TrimSpace(r.FormValue("freight_cost_per_m3")); raw != "" {
		if v, err := strconv.ParseFloat(raw, 64); err == nil && v >= 0 {
			freightPerM3 = v
		}
	}
	brokerFee := settings.BrokerFeePercent
	if raw := strings.TrimSpace(r.FormValue("broker_fee_percent")); raw != "" {
		if v, err := strconv.ParseFloat(raw, 64); err == nil && v >= 0 {
			brokerFee = v
		}
	}
	salesTax := settings.SalesTaxPercent
	if raw := strings.TrimSpace(r.FormValue("sales_tax_percent")); raw != "" {
		if v, err := strconv.ParseFloat(raw, 64); err == nil && v >= 0 {
			salesTax = v
		}
	}
	sellOverride := 0.0
	if raw := strings.TrimSpace(r.FormValue("product_sell_override")); raw != "" {
		if v, err := strconv.ParseFloat(raw, 64); err == nil && v > 0 {
			sellOverride = v
		}
	}
	extraSetup := 0.0
	if raw := strings.TrimSpace(r.FormValue("extra_setup_cost")); raw != "" {
		if v, err := strconv.ParseFloat(raw, 64); err == nil && v >= 0 {
			extraSetup = v
		}
	}
	// Auto relists by hub competition: Jita (2x) vs calmer Empire hubs (1x)
	estRelists := 1
	if strings.EqualFold(hubKey, "jita") {
		estRelists = 2
	}
	if raw := strings.TrimSpace(r.FormValue("estimated_relists")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 0 {
			estRelists = v
		}
	}
	batchesPerWeek := 4
	if raw := strings.TrimSpace(r.FormValue("batches_per_week")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 1 {
			batchesPerWeek = v
		}
	}

	in := domain.BlueprintCalcInput{
		BlueprintTypeID:     int32(bpID),
		Runs:                parseIntDefault(r.FormValue("runs"), 1),
		ME:                  parseIntDefault(r.FormValue("me"), 10),
		TE:                  parseIntDefault(r.FormValue("te"), 20),
		StructureKey:        strings.TrimSpace(r.FormValue("structure_key")),
		HubKey:              hubKey,
		MaterialPriceSource: strings.TrimSpace(r.FormValue("material_price_source")),
		MakeBuyOverrides:    domain.ParseMakeBuyOverrides(r.FormValue("make_buy_overrides")),
		ProductSellOverride: sellOverride,
		ExtraSetupCost:      extraSetup,
		SystemCostIndex:     sci,
		FacilityTaxPercent:  settings.FacilityTaxPercent,
		FreightCostPerM3:    freightPerM3,
		BrokerFeePercent:    brokerFee,
		SalesTaxPercent:     salesTax,
		RelistFeePercent:    settings.RelistFeePercent,
		EstimatedRelists:    estRelists,
		BatchesPerWeek:      batchesPerWeek,
	}

	res, err := h.svc.CalculateBlueprintProfit(r.Context(), in)
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	if h.renderer != nil {
		_ = h.renderer.RenderManufactureTreePartial(w, r, res, errMsg)
		return
	}
	if err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": errMsg})
		return
	}
	h.writeJSON(w, http.StatusOK, res)
}

// HandleSDEImport triggers the asynchronous 1-click Official CCP SDE (JSONL) import from /settings.
func (h *Handlers) HandleSDEImport(w http.ResponseWriter, r *http.Request) {
	started, err := h.svc.StartOfficialCCPSDEImport()
	if err != nil {
		if IsHTMX(r) {
			TriggerToast(w, "error", "Failed to start Official CCP SDE import: "+err.Error())
		}
	} else if started && IsHTMX(r) {
		TriggerToast(w, "info", "Official CCP SDE (JSONL) import started in background...")
	}

	status := h.svc.GetSDEImportStatus(r.Context())
	if h.renderer != nil && IsHTMX(r) {
		_ = h.renderer.RenderSDEStatusPartial(w, r, status)
		return
	}
	h.writeJSON(w, http.StatusOK, status)
}

// HandleSDEStatus returns the live progress and installed build counts of the Official CCP SDE importer.
func (h *Handlers) HandleSDEStatus(w http.ResponseWriter, r *http.Request) {
	status := h.svc.GetSDEImportStatus(r.Context())
	if h.renderer != nil && IsHTMX(r) {
		_ = h.renderer.RenderSDEStatusPartial(w, r, status)
		return
	}
	h.writeJSON(w, http.StatusOK, status)
}

func parseIntDefault(raw string, fallback int) int {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return v
}

func parseFloatDefault(raw string, fallback float64) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return fallback
	}
	return v
}
