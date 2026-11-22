package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"aura-forge/internal/domain"
)

const maxAgentToolIterations = 5

// AIChatTurnInput bundles a user prompt sent to the Level 4 Aura AI Copilot Agent.
type AIChatTurnInput struct {
	ConversationID int64  `json:"conversation_id"`
	Prompt         string `json:"prompt"`
	ContextPage    string `json:"context_page"`
	ContextSummary string `json:"context_summary,omitempty"`
}

// AIChatTurnResult returns the persisted conversation ID, full message list, and latest assistant message.
type AIChatTurnResult struct {
	ConversationID int64              `json:"conversation_id"`
	Messages       []domain.AIMessage `json:"messages"`
	LatestReply    *domain.AIMessage  `json:"latest_reply,omitempty"`
}

func currentActorIDAndAdmin(ctx context.Context) (int64, bool) {
	u := domain.AppUserFromContext(ctx)
	if u == nil {
		return 0, true // Auth-less test mode
	}
	return u.ID, u.IsAdmin()
}

// TestAIConnection verifies the configured OpenAI-compatible AI provider connectivity.
func (s *CopilotService) TestAIConnection(ctx context.Context) (string, error) {
	if s.aiProvider == nil || !s.aiProvider.IsEnabled() {
		return "", fmt.Errorf("Aura AI is disabled — configure AI_API_KEY, AI_BASE_URL, and AI_MODEL in .env")
	}
	return s.aiProvider.Ping(ctx)
}

// GetPilotMemory retrieves the current user's persistent AI playstyle & risk preferences.
func (s *CopilotService) GetPilotMemory(ctx context.Context) *domain.AIPilotMemory {
	uid, _ := currentActorIDAndAdmin(ctx)
	if s.aiRepo == nil {
		return &domain.AIPilotMemory{
			UserID:        uid,
			RiskTolerance: "balanced",
			DailyPlaytime: "1-2 hours/day",
		}
	}
	mem, err := s.aiRepo.GetPilotMemory(ctx, uid)
	if err != nil || mem == nil {
		return &domain.AIPilotMemory{
			UserID:        uid,
			RiskTolerance: "balanced",
			DailyPlaytime: "1-2 hours/day",
		}
	}
	return mem
}

// SavePilotMemory persists the user's AI playstyle & risk preferences.
func (s *CopilotService) SavePilotMemory(ctx context.Context, notes, riskTolerance, dailyPlaytime string) (*domain.AIPilotMemory, error) {
	uid, _ := currentActorIDAndAdmin(ctx)
	mem := &domain.AIPilotMemory{
		UserID:         uid,
		PlaystyleNotes: strings.TrimSpace(notes),
		RiskTolerance:  strings.TrimSpace(riskTolerance),
		DailyPlaytime:  strings.TrimSpace(dailyPlaytime),
	}
	if s.aiRepo != nil {
		if err := s.aiRepo.SavePilotMemory(ctx, mem); err != nil {
			return nil, err
		}
	}
	return mem, nil
}

// ListAIConversations returns recent Aura Copilot chat threads for the active user.
func (s *CopilotService) ListAIConversations(ctx context.Context) ([]domain.AIConversation, error) {
	if s.aiRepo == nil {
		return []domain.AIConversation{}, nil
	}
	uid, _ := currentActorIDAndAdmin(ctx)
	return s.aiRepo.ListConversations(ctx, uid, 20)
}

// GetAIConversationMessages loads messages for one conversation owned by the active user.
func (s *CopilotService) GetAIConversationMessages(ctx context.Context, convID int64) ([]domain.AIMessage, error) {
	if s.aiRepo == nil || convID <= 0 {
		return []domain.AIMessage{}, nil
	}
	uid, _ := currentActorIDAndAdmin(ctx)
	conv, err := s.aiRepo.GetConversation(ctx, convID, uid)
	if err != nil || conv == nil {
		return []domain.AIMessage{}, err
	}
	return s.aiRepo.ListMessages(ctx, conv.ID, 40)
}

// DeleteAIConversation removes a conversation owned by the active user.
func (s *CopilotService) DeleteAIConversation(ctx context.Context, convID int64) error {
	if s.aiRepo == nil || convID <= 0 {
		return nil
	}
	uid, _ := currentActorIDAndAdmin(ctx)
	return s.aiRepo.DeleteConversation(ctx, convID, uid)
}

// AskAuraCopilot executes the Level 4 Agentic Copilot Loop:
// 1. Loads pilot settings, active Trade Hub, Playbook Memory, and conversation history.
// 2. Runs up to 5 OpenAI-compatible tool-calling iterations invoking live Go backend tools.
// 3. Collects structured Human-in-the-Loop Action Cards (never silently mutating DB state).
// 4. Persists the user prompt and assistant response with tool traces & action cards.
func (s *CopilotService) AskAuraCopilot(ctx context.Context, in AIChatTurnInput) (*AIChatTurnResult, error) {
	if s.aiProvider == nil || !s.aiProvider.IsEnabled() {
		return nil, fmt.Errorf("Aura AI is not configured — add AI_API_KEY, AI_BASE_URL, and AI_MODEL in .env and restart the server")
	}
	prompt := strings.TrimSpace(in.Prompt)
	if prompt == "" {
		return nil, fmt.Errorf("please enter a question for Aura Copilot")
	}

	uid, isAdmin := currentActorIDAndAdmin(ctx)
	settings, _ := s.GetSettings(ctx)
	hub := domain.LookupTradeHub("jita")
	if settings != nil {
		hub = settings.ActiveTradeHub()
	}
	pilotMem := s.GetPilotMemory(ctx)

	// Ensure or create conversation
	convID := in.ConversationID
	var history []domain.AIMessage
	if s.aiRepo != nil {
		if convID > 0 {
			conv, _ := s.aiRepo.GetConversation(ctx, convID, uid)
			if conv != nil {
				history, _ = s.aiRepo.ListMessages(ctx, conv.ID, 12)
			} else {
				convID = 0
			}
		}
		if convID == 0 {
			title := prompt
			if len(title) > 72 {
				title = title[:69] + "..."
			}
			newConv := &domain.AIConversation{
				UserID:      uid,
				Title:       title,
				ContextPage: in.ContextPage,
			}
			if err := s.aiRepo.CreateConversation(ctx, newConv); err == nil {
				convID = newConv.ID
			}
		}
	}

	userMsg := domain.AIMessage{
		ConversationID: convID,
		Role:           "user",
		Content:        prompt,
		CreatedAt:      time.Now(),
	}
	if s.aiRepo != nil && convID > 0 {
		_ = s.aiRepo.AppendMessage(ctx, &userMsg)
	}

	// Build system prompt with clear guardrails and pilot context
	sysPrompt := buildAuraSystemPrompt(isAdmin, hub, settings, pilotMem, in.ContextPage, in.ContextSummary)
	messages := make([]domain.AIChatMessage, 0, len(history)+6)
	messages = append(messages, domain.AIChatMessage{Role: "system", Content: sysPrompt})
	for _, h := range history {
		messages = append(messages, domain.AIChatMessage{Role: h.Role, Content: h.Content})
	}
	messages = append(messages, domain.AIChatMessage{Role: "user", Content: prompt})

	tools := buildAuraToolDefinitions(isAdmin)
	var (
		traces      []domain.AIToolTrace
		actionCards []domain.AIActionCard
		finalText   string
	)

	// Level 4 Agentic Tool-Calling Loop
	for step := 0; step < maxAgentToolIterations; step++ {
		req := domain.AIChatRequest{
			Messages:    messages,
			Tools:       tools,
			Temperature: 0.25,
		}
		// On final step, force synthesis without further tool calls
		if step == maxAgentToolIterations-1 {
			req.Tools = nil
		}

		resp, err := s.aiProvider.ChatCompletion(ctx, req)
		if err != nil {
			return nil, err
		}

		if len(resp.ToolCalls) == 0 {
			finalText = resp.Content
			break
		}

		// Record assistant's tool call request in the conversation trace
		messages = append(messages, domain.AIChatMessage{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		for _, tc := range resp.ToolCalls {
			toolOut, trace, proposedCard := s.executeAuraTool(ctx, isAdmin, hub, tc)
			traces = append(traces, trace)
			if proposedCard != nil {
				actionCards = append(actionCards, *proposedCard)
			}
			messages = append(messages, domain.AIChatMessage{
				Role:       "tool",
				ToolCallID: tc.ID,
				Name:       tc.Function.Name,
				Content:    toolOut,
			})
		}
	}

	if strings.TrimSpace(finalText) == "" {
		finalText = "I gathered the live market and industry telemetry above, but the AI model returned an empty summary. Please try asking a more specific follow-up question."
	}

	assistantMsg := domain.AIMessage{
		ConversationID: convID,
		Role:           "assistant",
		Content:        finalText,
		ToolTraces:     traces,
		ActionCards:    actionCards,
		CreatedAt:      time.Now(),
	}
	if s.aiRepo != nil && convID > 0 {
		_ = s.aiRepo.AppendMessage(ctx, &assistantMsg)
	}

	var allMsgs []domain.AIMessage
	if s.aiRepo != nil && convID > 0 {
		allMsgs, _ = s.aiRepo.ListMessages(ctx, convID, 40)
	} else {
		allMsgs = append(history, userMsg, assistantMsg)
	}

	return &AIChatTurnResult{
		ConversationID: convID,
		Messages:       allMsgs,
		LatestReply:    &assistantMsg,
	}, nil
}

func buildAuraSystemPrompt(
	isAdmin bool,
	hub domain.TradeHub,
	settings *domain.UserSettings,
	mem *domain.AIPilotMemory,
	contextPage, contextSummary string,
) string {
	var b strings.Builder
	b.WriteString("You are Aura, an elite Level-4 Agentic EVE Online Market & Industry Copilot inside aura-forge.\n")
	b.WriteString("Mission: Guide a beginner-to-scaling EVE Online pilot toward 100B+ ISK in TOTAL WEALTH (Net Worth — NOT starting capital) with margin safety, capital velocity, and clear actionable steps.\n\n")
	b.WriteString("STRICT RULES:\n")
	b.WriteString("1. LANGUAGE: Always reply in the SAME language the user writes in (Indonesian if they write Indonesian, English if they write English).\n")
	b.WriteString("2. NEVER GUESS NUMBERS: Always call your live tools to check prices, order books, opportunities, blueprints, catalog, batches, PI, or appraisal before stating ISK figures.\n")
	b.WriteString("3. BEGINNER SAFETY FIRST: Protect against slow movers, price spikes (>25% above 7d avg), unprofitable undercuts (check Safe Relist Floor), >35% single-item capital lock-up, and High-Sec Gank Risk (>200M ISK per T1 hauler trip). Always preserve a 10-15% Cash Buffer.\n")
	b.WriteString("4. HUMAN-IN-THE-LOOP ACTIONS: When you recommend a concrete next step (adding a viable item to Catalog, recording a Draft Batch, or opening Manufacture/Marketplace), call `propose_action_card` so the user gets a 1-click button.\n")
	b.WriteString("5. CONCISE & STRUCTURED: Use crisp Markdown bullets/bold numbers, clear verdicts (SAFE / CAUTION / AVOID), and concrete next actions.\n\n")

	b.WriteString(fmt.Sprintf("CURRENT CONTEXT:\n- Active Trade Hub: %s (%s, key=%s)\n- Active Page: %s\n- User Role: %s\n",
		hub.Name, hub.RegionName, hub.Key,
		func() string {
			if contextPage != "" {
				return contextPage
			}
			return "dashboard"
		}(),
		func() string {
			if isAdmin {
				return "ADMIN (full business & research tools)"
			}
			return "USER (research-only tools)"
		}(),
	))
	if settings != nil {
		b.WriteString(fmt.Sprintf("- Tax Profile: Broker Fee %.2f%%, Sales Tax %.2f%%, Relist Fee %.2f%%, SCI %.2f%%, Facility Tax %.2f%%\n",
			settings.BrokerFeePercent, settings.SalesTaxPercent, settings.RelistFeePercent, settings.SystemCostIndex*100, settings.FacilityTaxPercent))
	}
	if mem != nil {
		b.WriteString(fmt.Sprintf("- Pilot Playbook Memory: Risk=%s | Playtime=%s", mem.RiskTolerance, mem.DailyPlaytime))
		if mem.PlaystyleNotes != "" {
			b.WriteString(" | Notes: " + mem.PlaystyleNotes)
		}
		b.WriteString("\n")
	}
	if strings.TrimSpace(contextSummary) != "" {
		b.WriteString("- Current Screen Data:\n" + strings.TrimSpace(contextSummary) + "\n")
	}
	return b.String()
}

func buildAuraToolDefinitions(isAdmin bool) []domain.AIToolDefinition {
	def := func(name, desc, schema string) domain.AIToolDefinition {
		return domain.AIToolDefinition{
			Type: "function",
			Function: domain.AIToolFunction{
				Name:        name,
				Description: desc,
				Parameters:  json.RawMessage(schema),
			},
		}
	}

	tools := make([]domain.AIToolDefinition, 0, 8)
	if isAdmin {
		tools = append(tools,
			def("get_business_snapshot",
				"Get the pilot's live business dashboard snapshot: available working capital, locked capital, realized profit, manufacturing/science slot usage, undercut count, PI colony expiry alerts, and Total Net Worth milestone progress.",
				`{"type":"object","properties":{}}`),
			def("check_catalog_and_batches",
				"List the pilot's Catalog items (with VIABLE/TRAP status, margin, ROI, daily volume) and recent Tracker batches (DRAFT, IN_PRODUCTION, LISTED, SLOW_MOVER, SOLD, undercut status).",
				`{"type":"object","properties":{"status_filter":{"type":"string","description":"Optional batch status filter: DRAFT, IN_PRODUCTION, LISTED, SLOW_MOVER, SOLD"}}}`),
			def("get_trade_ledger",
				"Get the pilot's closed-loop Buy & Sell Ledger and Realized P&L analytics: Cash In/Out, Realized Net Profit & ROI (weighted-average cost basis), Top Cash Cows, Capital Drains, Fee & Tax Leak breakdown (Broker/Relist/Sales/Industry/PI), and Active & Historical Order Fill Rates.",
				`{"type":"object","properties":{"period_days":{"type":"integer","description":"Lookback window in days (default 30; use 0 for all-time)"}}}`),
		)
	}

	tools = append(tools,
		def("search_marketplace",
			"Inspect live EVE Online Marketplace data for one item at a Trade Hub: lowest sell, highest buy, 5-hub price comparison, 7d/30d price trend & spike signal, daily volume, Net Flip Margin, top sell walls, and SDE Craft-vs-Buy comparison.",
			`{"type":"object","properties":{"item":{"type":"string","description":"Exact or partial EVE item name (e.g. Hobgoblin I, Caracal, Coolant, Damage Control II)"},"hub":{"type":"string","description":"Optional hub key: jita, amarr, dodixie, rens, hek (defaults to pilot's Primary Trade Hub)"}},"required":["item"]}`),
		def("scan_opportunities",
			"Run the live Opportunity Scanner to find profitable EVE items at a Trade Hub.",
			`{"type":"object","properties":{"mode":{"type":"string","enum":["manufacturing","station_trading","arbitrage"],"description":"Scanner mode"},"hub":{"type":"string","description":"Source/primary hub key: jita, amarr, dodixie, rens, hek"},"dest_hub":{"type":"string","description":"Destination hub key for arbitrage mode (e.g. amarr)"},"capital_isk":{"type":"number","description":"Available capital in ISK to size batches/trips"}},"required":["mode"]}`),
		def("inspect_manufacture_tree",
			"Inspect a CCP SDE Blueprint recipe tree with live Trade Hub material costs, MAKE vs BUY optimal savings, total job install cost, output sell value, and cargo volume m3.",
			`{"type":"object","properties":{"blueprint":{"type":"string","description":"Product or blueprint name (e.g. Caracal, Hobgoblin II, Rifter)"},"hub":{"type":"string","description":"Trade hub key: jita, amarr, dodixie, rens, hek"},"runs":{"type":"integer","description":"Number of runs (default 1)"},"me":{"type":"integer","description":"Material Efficiency 0..10 (default 10 for T1, 2 for T2)"}},"required":["blueprint"]}`),
		def("simulate_pi_sales",
			"Simulate selling one or more Planetary Industry (PI) P1-P4 items for custom unit quantities at a Trade Hub, including POCO tax, Broker/Sales tax, and Epithal cargo trips.",
			`{"type":"object","properties":{"items_text":{"type":"string","description":"Newline-separated PI items and quantities e.g. 'Coolant\t2000\nMechanical Parts\t1500'"},"hub":{"type":"string","description":"Trade hub key"},"cost_mode":{"type":"string","enum":["self_extracted","buy_inputs"]},"sell_mode":{"type":"string","enum":["sell_order","instant_buy"]},"poco_tax":{"type":"number","description":"POCO tax percent (default 5)"}},"required":["items_text"]}`),
		def("appraise_clipboard_items",
			"Appraise pasted EVE Online inventory/cargo/loot/fit text at a Trade Hub for Total Sell, Total Buy, post-tax net ISK, Buyback value, volume m3, and High-Sec Gank Risk.",
			`{"type":"object","properties":{"paste_text":{"type":"string","description":"Raw EVE item lines with quantities"},"hub":{"type":"string","description":"Trade hub key"},"buyback_pct":{"type":"number","description":"Buyback percentage (default 90)"}},"required":["paste_text"]}`),
		def("propose_action_card",
			"Attach a Human-in-the-Loop 1-click Action Card under your reply so the user can approve and execute a concrete action (open a page, or for Admins: add an item to Catalog or create a Draft Batch/Trade Flip).",
			`{"type":"object","properties":{"kind":{"type":"string","enum":["open_link","add_catalog","create_batch"]},"entry_type":{"type":"string","enum":["MANUFACTURING","TRADE"],"description":"For create_batch: MANUFACTURING for production runs or TRADE for station trading / arbitrage flips"},"title":{"type":"string"},"description":{"type":"string"},"button_label":{"type":"string"},"item_name":{"type":"string"},"type_id":{"type":"integer"},"category":{"type":"string"},"runs":{"type":"integer"},"output_qty":{"type":"integer"},"unit_sell_price":{"type":"number"},"material_cost":{"type":"number","description":"Total build cost (MFG) or total buy cost (TRADE)"},"daily_volume":{"type":"number"},"url":{"type":"string","description":"For open_link kind, relative app path like /manufacture, /marketplace?q=Caracal&hub=amarr, /scanner, /tracker, /pi"}},"required":["kind","title","button_label"]}`),
	)
	return tools
}

func (s *CopilotService) executeAuraTool(
	ctx context.Context,
	isAdmin bool,
	defaultHub domain.TradeHub,
	tc domain.AIToolCall,
) (string, domain.AIToolTrace, *domain.AIActionCard) {
	name := tc.Function.Name
	argsRaw := strings.TrimSpace(tc.Function.Arguments)
	if argsRaw == "" {
		argsRaw = "{}"
	}
	trace := domain.AIToolTrace{
		ToolName: name,
		ArgsJSON: argsRaw,
	}

	toJSON := func(v any) string {
		b, _ := json.Marshal(v)
		s := string(b)
		if len(s) > 6500 {
			s = s[:6500] + "...(truncated)"
		}
		return s
	}

	switch name {
	case "get_business_snapshot":
		if !isAdmin {
			trace.Error = "admin only"
			return `{"error":"get_business_snapshot is restricted to Admin accounts"}`, trace, nil
		}
		dash, err := s.GetDashboard(ctx)
		if err != nil {
			trace.Error = err.Error()
			return toJSON(map[string]string{"error": err.Error()}), trace, nil
		}
		trace.Summary = fmt.Sprintf("Capital %s | Hub %s | %d/%d MFG slots | %d undercut",
			fmtCompactISK(dash.AvailableCapital), dash.ActiveHub.Name, dash.MfgSlotsUsed, dash.MfgSlotsTotal, dash.UndercutBatchesCount)
		return toJSON(map[string]any{
			"active_hub":              dash.ActiveHub.Name,
			"available_capital_isk":   dash.AvailableCapital,
			"locked_capital_isk":      dash.LockedCapital,
			"realized_profit_isk":     dash.RealizedProfit,
			"current_net_worth_isk":   dash.CurrentNetWorth,
			"next_milestone":          dash.NextMilestoneLabel,
			"next_milestone_pct":      dash.NextMilestonePct,
			"mfg_slots_used":          dash.MfgSlotsUsed,
			"mfg_slots_total":         dash.MfgSlotsTotal,
			"sci_slots_used":          dash.SciSlotsUsed,
			"sci_slots_total":         dash.SciSlotsTotal,
			"catalog_viable_count":    dash.LayakItemsCount,
			"catalog_trap_count":      dash.JebakanItemsCount,
			"active_batches_count":    dash.ActiveBatchesCount,
			"undercut_batches_count":  dash.UndercutBatchesCount,
			"pi_expired_colonies":     dash.ExpiredColoniesCount,
			"pi_expiring_colonies":    dash.ExpiringColoniesCount,
			"captured_wealth_today":   dash.CapturedWealthToday,
			"copilot_urgent_actions":  dash.CopilotActions,
		}), trace, nil

	case "check_catalog_and_batches":
		if !isAdmin {
			trace.Error = "admin only"
			return `{"error":"check_catalog_and_batches is restricted to Admin accounts"}`, trace, nil
		}
		var args struct {
			StatusFilter string `json:"status_filter"`
		}
		_ = json.Unmarshal([]byte(argsRaw), &args)

		items, _ := s.ListCatalog(ctx, "", "", "")
		batches, _ := s.ListBatches(ctx, args.StatusFilter)

		type catLite struct {
			TypeID     int32   `json:"type_id"`
			Name       string  `json:"name"`
			Category   string  `json:"category"`
			SellPrice  float64 `json:"sell_price"`
			MatCost    float64 `json:"material_cost"`
			BatchQty   int     `json:"batch_qty"`
			DailyVol   float64 `json:"daily_volume"`
			Status     string  `json:"status"`
			MarginPct  float64 `json:"margin_pct"`
			ROIPct     float64 `json:"roi_pct"`
			TrapReason string  `json:"trap_reason,omitempty"`
		}
		catOut := make([]catLite, 0, minInt(len(items), 18))
		for i := 0; i < len(items) && i < 18; i++ {
			it := items[i]
			catOut = append(catOut, catLite{
				TypeID:     it.TypeID,
				Name:       it.Name,
				Category:   string(it.Category),
				SellPrice:  it.EstimatedJitaSell,
				MatCost:    it.MaterialCost,
				BatchQty:   it.BatchOutputQty,
				DailyVol:   it.DailyVolume,
				Status:     string(it.TrapAnalysis.Status),
				MarginPct:  it.TrapAnalysis.EstimatedMarginPct,
				ROIPct:     it.TrapAnalysis.EstimatedROIPct,
				TrapReason: it.TrapAnalysis.Recommendation,
			})
		}

		type batchLite struct {
			ID         int64   `json:"id"`
			EntryType  string  `json:"entry_type"`
			ItemName   string  `json:"item_name"`
			Status     string  `json:"status"`
			Units      int     `json:"units"`
			Cost       float64 `json:"cost"`
			SellPrice  float64 `json:"sell_price"`
			MarketNow  float64 `json:"market_now"`
			IsUndercut bool    `json:"is_undercut"`
			SafeRelist float64 `json:"safe_relist_floor"`
			NetProfit  float64 `json:"net_profit"`
		}
		batchOut := make([]batchLite, 0, minInt(len(batches), 18))
		for i := 0; i < len(batches) && i < 18; i++ {
			b := batches[i]
			batchOut = append(batchOut, batchLite{
				ID:         b.ID,
				EntryType:  string(b.NormalizedEntryType()),
				ItemName:   b.ItemName,
				Status:     string(b.Status),
				Units:      b.UnitsOutput,
				Cost:       b.ActualProductionCost,
				SellPrice:  b.ActualSellPrice,
				MarketNow:  b.CurrentMarketPrice,
				IsUndercut: b.IsUndercut,
				SafeRelist: b.SafeRelistPrice,
				NetProfit:  b.NetProfit,
			})
		}

		trace.Summary = fmt.Sprintf("Checked %d catalog items & %d batches", len(items), len(batches))
		return toJSON(map[string]any{
			"catalog_total": len(items),
			"catalog_items": catOut,
			"batches_total": len(batches),
			"batches":       batchOut,
		}), trace, nil

	case "get_trade_ledger":
		if !isAdmin {
			trace.Error = "admin only"
			return `{"error":"get_trade_ledger is restricted to Admin accounts"}`, trace, nil
		}
		var args struct {
			PeriodDays int `json:"period_days"`
		}
		_ = json.Unmarshal([]byte(argsRaw), &args)
		days := args.PeriodDays
		if days == 0 && !strings.Contains(argsRaw, `"period_days"`) {
			days = 30
		}
		view, err := s.GetTradeLedgerView(ctx, days, "", "", false)
		if err != nil {
			trace.Error = err.Error()
			return toJSON(map[string]string{"error": err.Error()}), trace, nil
		}
		rep := view.Report
		trace.Summary = fmt.Sprintf("Ledger (%dd): Realized %s | Fees %s | %d items",
			days, fmtCompactISK(rep.RealizedProfitISK), fmtCompactISK(rep.FeeLeak.TotalFeesISK), len(rep.Items))
		return toJSON(map[string]any{
			"period_days":         days,
			"buy_count":           rep.TotalBuyCount,
			"sell_count":          rep.TotalSellCount,
			"total_cash_out_isk":  rep.TotalCashOutISK,
			"total_cash_in_isk":   rep.TotalCashInISK,
			"net_cash_flow_isk":   rep.NetCashFlowISK,
			"realized_profit_isk": rep.RealizedProfitISK,
			"realized_roi_pct":    rep.RealizedROIPct,
			"open_inventory_isk":  rep.OpenInventoryISK,
			"fee_leak":            rep.FeeLeak,
			"orders_summary":      rep.Orders,
			"top_cash_cows":       rep.TopCashCows,
			"capital_drains":      rep.CapitalDrains,
		}), trace, nil

	case "search_marketplace":
		var args struct {
			Item string `json:"item"`
			Hub  string `json:"hub"`
		}
		_ = json.Unmarshal([]byte(argsRaw), &args)
		hubKey := strings.TrimSpace(args.Hub)
		if hubKey == "" {
			hubKey = defaultHub.Key
		}
		snap, err := s.GetMarketplaceSnapshot(ctx, args.Item, hubKey, true)
		if err != nil {
			trace.Error = err.Error()
			return toJSON(map[string]string{"error": err.Error()}), trace, nil
		}
		topSells := snap.SellOrders
		if len(topSells) > 5 {
			topSells = topSells[:5]
		}
		trace.Summary = fmt.Sprintf("%s @ %s: Sell %s / Buy %s (%s)",
			snap.TypeName, snap.SelectedHub.Name, fmtCompactISK(snap.LowestSell), fmtCompactISK(snap.HighestBuy), snap.PriceSignal)
		bestArbRoute := ""
		if snap.BestArbOriginHub.Name != "" && snap.BestArbDestHub.Name != "" {
			bestArbRoute = snap.BestArbOriginHub.Name + " → " + snap.BestArbDestHub.Name
		}
		return toJSON(map[string]any{
			"type_id":             snap.TypeID,
			"name":                snap.TypeName,
			"hub":                 snap.SelectedHub.Name,
			"lowest_sell":         snap.LowestSell,
			"highest_buy":         snap.HighestBuy,
			"spread_pct":          snap.SpreadPercent,
			"net_flip_margin":     snap.NetFlipMarginPct,
			"avg_daily_vol_7d":    snap.AvgDailyVolume7d,
			"avg_7d_price":        snap.Avg7dPrice,
			"price_signal":        snap.PriceSignal,
			"change_7d_pct":       snap.Change7dPct,
			"has_sde_blueprint":   snap.HasBlueprint,
			"sde_craft_unit":      snap.EstCraftUnitCost,
			"sde_craft_savings":   snap.CraftSavingsPct,
			"best_arb_route":      bestArbRoute,
			"best_arb_margin_pct": snap.BestArbNetMarginPct,
			"hub_comparison":      snap.HubComparisons,
			"top_5_sell_orders":   topSells,
		}), trace, nil

	case "scan_opportunities":
		var args struct {
			Mode       string  `json:"mode"`
			Hub        string  `json:"hub"`
			DestHub    string  `json:"dest_hub"`
			CapitalISK float64 `json:"capital_isk"`
		}
		_ = json.Unmarshal([]byte(argsRaw), &args)
		hubKey := strings.TrimSpace(args.Hub)
		if hubKey == "" {
			hubKey = defaultHub.Key
		}
		capISK := args.CapitalISK
		if capISK <= 0 {
			if cfg, err := s.GetSettings(ctx); err == nil && cfg != nil {
				capISK = cfg.StartingCapital
			}
		}

		switch strings.ToLower(strings.TrimSpace(args.Mode)) {
		case "station_trading":
			rows, err := s.ScanStationTradingOpportunitiesV2(ctx, hubKey, "", true)
			if err != nil {
				trace.Error = err.Error()
				return toJSON(map[string]string{"error": err.Error()}), trace, nil
			}
			if len(rows) > 8 {
				rows = rows[:8]
			}
			trace.Summary = fmt.Sprintf("Station Trading (%s): %d top flips", hubKey, len(rows))
			return toJSON(rows), trace, nil

		case "arbitrage":
			destKey := strings.TrimSpace(args.DestHub)
			if destKey == "" || destKey == hubKey {
				destKey = "amarr"
				if hubKey == "amarr" {
					destKey = "jita"
				}
			}
			rows, err := s.ScanInterHubArbitrageOpportunitiesV2(ctx, hubKey, destKey, "", 5000, capISK, true)
			if err != nil {
				trace.Error = err.Error()
				return toJSON(map[string]string{"error": err.Error()}), trace, nil
			}
			if len(rows) > 8 {
				rows = rows[:8]
			}
			trace.Summary = fmt.Sprintf("Arbitrage %s → %s: %d routes", hubKey, destKey, len(rows))
			return toJSON(rows), trace, nil

		default: // manufacturing
			rows, err := s.ScanManufacturingOpportunitiesV2(ctx, ManufacturingScanRequest{
				HubKey:              hubKey,
				Runs:                10,
				ME:                  10,
				TE:                  20,
				StructureKey:        "npc_station",
				MaterialPriceSource: "jita_sell",
				ProfitableOnly:      true,
				FitsCapitalOnly:     true,
				SortBy:              "isk_hr",
			})
			if err != nil {
				trace.Error = err.Error()
				return toJSON(map[string]string{"error": err.Error()}), trace, nil
			}
			if len(rows) > 8 {
				rows = rows[:8]
			}
			trace.Summary = fmt.Sprintf("Manufacturing (%s): %d top blueprints", hubKey, len(rows))
			return toJSON(rows), trace, nil
		}

	case "inspect_manufacture_tree":
		var args struct {
			Blueprint string `json:"blueprint"`
			Hub       string `json:"hub"`
			Runs      int    `json:"runs"`
			ME        int    `json:"me"`
		}
		_ = json.Unmarshal([]byte(argsRaw), &args)
		if args.Runs <= 0 {
			args.Runs = 1
		}
		hubKey := strings.TrimSpace(args.Hub)
		if hubKey == "" {
			hubKey = defaultHub.Key
		}

		bps, err := s.SearchBlueprints(ctx, strings.TrimSpace(args.Blueprint), "", 5)
		if err != nil || len(bps) == 0 {
			trace.Error = "blueprint not found"
			return toJSON(map[string]string{"error": "blueprint not found in CCP SDE for: " + args.Blueprint}), trace, nil
		}
		chosen := bps[0]
		for _, cand := range bps {
			if strings.EqualFold(cand.ProductName, strings.TrimSpace(args.Blueprint)) {
				chosen = cand
				break
			}
		}
		me := args.ME
		if me == 0 && !strings.Contains(argsRaw, `"me"`) {
			if chosen.TechLevel >= 2 {
				me = 2
			} else {
				me = 10
			}
		}
		settings, _ := s.GetSettings(ctx)
		in := domain.BlueprintCalcInput{
			BlueprintTypeID:     chosen.BlueprintTypeID,
			Runs:                args.Runs,
			ME:                  me,
			TE:                  20,
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
		res, err := s.CalculateBlueprintProfit(ctx, in)
		if err != nil {
			trace.Error = err.Error()
			return toJSON(map[string]string{"error": err.Error()}), trace, nil
		}
		optCost := res.TotalMaterialCost
		buyDirect := res.TotalMaterialCost
		volM3 := res.TotalMaterialVolumeM3
		if res.IndustryTree != nil {
			optCost = res.IndustryTree.OptimalCost
			buyDirect = res.IndustryTree.BuyAllDirectCost
			volM3 = res.IndustryTree.OptimalVolumeM3
		}
		trace.Summary = fmt.Sprintf("%s (%dx @ %s): Recipe %s vs Sell %s",
			res.Blueprint.ProductName, res.Runs, res.PriceHub.Name, fmtCompactISK(optCost), fmtCompactISK(res.ProductUnitSellPrice*float64(res.TotalOutputQty)))
		return toJSON(map[string]any{
			"blueprint_type_id":     res.Blueprint.BlueprintTypeID,
			"product_type_id":       res.Blueprint.ProductTypeID,
			"product_name":          res.Blueprint.ProductName,
			"category":              res.Blueprint.Category,
			"tech_level":            res.Blueprint.TechLevel,
			"hub":                   res.PriceHub.Name,
			"runs":                  res.Runs,
			"total_output_qty":      res.TotalOutputQty,
			"unit_sell_price":       res.ProductUnitSellPrice,
			"optimal_recipe_cost":   optCost,
			"buy_all_direct_cost":   buyDirect,
			"net_profit_after_tax":  res.Profit.NetProfitPerBatch,
			"net_margin_pct":        res.Profit.NetMarginPercent,
			"roi_pct":               res.Profit.ROIPercent,
			"isk_per_hour":          res.ISKPerHour,
			"duration":              res.DurationFormatted,
			"cargo_volume_m3":       volM3,
			"materials_count":       len(res.Materials),
		}), trace, nil

	case "simulate_pi_sales":
		var args struct {
			ItemsText string  `json:"items_text"`
			Hub       string  `json:"hub"`
			CostMode  string  `json:"cost_mode"`
			SellMode  string  `json:"sell_mode"`
			POCOTax   float64 `json:"poco_tax"`
		}
		_ = json.Unmarshal([]byte(argsRaw), &args)
		hubKey := strings.TrimSpace(args.Hub)
		if hubKey == "" {
			hubKey = defaultHub.Key
		}
		if args.POCOTax <= 0 && !strings.Contains(argsRaw, `"poco_tax"`) {
			args.POCOTax = 5.0
		}
		parsed, unmatched := domain.ParsePIInventoryPaste(args.ItemsText)
		res, err := s.SimulatePISales(ctx, parsed, args.CostMode, args.SellMode, args.POCOTax, hubKey)
		if err != nil {
			trace.Error = err.Error()
			return toJSON(map[string]string{"error": err.Error()}), trace, nil
		}
		trace.Summary = fmt.Sprintf("PI Sales (%s): Net %s (%.1f m³, %d Epithal trip)",
			res.Options.Hub.Name, fmtCompactISK(res.TotalNetProfit), res.TotalVolumeM3, res.EpithalTrips)
		return toJSON(map[string]any{
			"result":    res,
			"unmatched": unmatched,
		}), trace, nil

	case "appraise_clipboard_items":
		var args struct {
			PasteText  string  `json:"paste_text"`
			Hub        string  `json:"hub"`
			BuybackPct float64 `json:"buyback_pct"`
		}
		_ = json.Unmarshal([]byte(argsRaw), &args)
		hubKey := strings.TrimSpace(args.Hub)
		if hubKey == "" {
			hubKey = defaultHub.Key
		}
		res, err := s.AppraiseEVEClipboard(ctx, args.PasteText, hubKey, args.BuybackPct)
		if err != nil {
			trace.Error = err.Error()
			return toJSON(map[string]string{"error": err.Error()}), trace, nil
		}
		trace.Summary = fmt.Sprintf("Appraised %d items @ %s: Sell %s / Buy %s",
			res.TotalDistinct, res.Hub.Name, fmtCompactISK(res.TotalSell), fmtCompactISK(res.TotalBuy))
		return toJSON(res), trace, nil

	case "propose_action_card":
		var args struct {
			Kind          string  `json:"kind"`
			EntryType     string  `json:"entry_type"`
			Title         string  `json:"title"`
			Description   string  `json:"description"`
			ButtonLabel   string  `json:"button_label"`
			ItemName      string  `json:"item_name"`
			TypeID        int32   `json:"type_id"`
			Category      string  `json:"category"`
			Runs          int     `json:"runs"`
			OutputQty     int     `json:"output_qty"`
			UnitSellPrice float64 `json:"unit_sell_price"`
			MaterialCost  float64 `json:"material_cost"`
			DailyVolume   float64 `json:"daily_volume"`
			URL           string  `json:"url"`
		}
		_ = json.Unmarshal([]byte(argsRaw), &args)

		card := &domain.AIActionCard{
			Kind:        args.Kind,
			Title:       strings.TrimSpace(args.Title),
			Description: strings.TrimSpace(args.Description),
			ButtonLabel: strings.TrimSpace(args.ButtonLabel),
		}

		switch args.Kind {
		case "add_catalog":
			if !isAdmin {
				return `{"status":"skipped","reason":"add_catalog is admin-only"}`, trace, nil
			}
			card.Method = "POST"
			card.URL = "/catalog/items"
			qty := args.OutputQty
			if qty <= 0 {
				qty = 10
			}
			cat := args.Category
			if cat == "" {
				cat = "Other"
			}
			vol := args.DailyVolume
			if vol <= 0 {
				vol = 100
			}
			card.Payload = map[string]string{
				"type_id":           fmt.Sprintf("%d", args.TypeID),
				"name":              args.ItemName,
				"category":          cat,
				"batch_output_qty":  fmt.Sprintf("%d", qty),
				"jita_sell":         fmt.Sprintf("%.2f", args.UnitSellPrice),
				"material_cost":     fmt.Sprintf("%.2f", args.MaterialCost),
				"daily_volume":      fmt.Sprintf("%.0f", vol),
				"is_manufacturable": "on",
			}
		case "create_batch":
			if !isAdmin {
				return `{"status":"skipped","reason":"create_batch is admin-only"}`, trace, nil
			}
			card.Method = "POST"
			card.URL = "/tracker/batches"
			entryType := strings.ToUpper(strings.TrimSpace(args.EntryType))
			if entryType != string(domain.BatchEntryTrade) {
				entryType = string(domain.BatchEntryManufacturing)
			}
			runs := args.Runs
			if runs <= 0 {
				runs = 1
			}
			qty := args.OutputQty
			if qty <= 0 {
				qty = runs
			}
			card.Payload = map[string]string{
				"entry_type":             entryType,
				"item_name":              args.ItemName,
				"runs":                   fmt.Sprintf("%d", runs),
				"units_output":           fmt.Sprintf("%d", qty),
				"actual_production_cost": fmt.Sprintf("%.2f", args.MaterialCost),
				"actual_sell_price":      fmt.Sprintf("%.2f", args.UnitSellPrice),
				"status":                 "DRAFT",
			}
		default:
			card.Kind = "open_link"
			card.Method = "GET"
			target := strings.TrimSpace(args.URL)
			if target == "" && args.ItemName != "" {
				target = "/marketplace?q=" + url.QueryEscape(args.ItemName) + "&hub=" + defaultHub.Key
			}
			if !strings.HasPrefix(target, "/") {
				target = "/"
			}
			card.URL = target
		}

		trace.Summary = "Proposed 1-click action: " + card.Title
		return `{"status":"action_card_attached"}`, trace, card

	default:
		trace.Error = "unknown tool"
		return fmt.Sprintf(`{"error":"unknown tool %q"}`, name), trace, nil
	}
}

func fmtCompactISK(v float64) string {
	switch {
	case v >= 1_000_000_000:
		return fmt.Sprintf("%.2fB ISK", v/1_000_000_000)
	case v >= 1_000_000:
		return fmt.Sprintf("%.2fM ISK", v/1_000_000)
	case v >= 1_000:
		return fmt.Sprintf("%.1fK ISK", v/1_000)
	default:
		return fmt.Sprintf("%.0f ISK", v)
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
