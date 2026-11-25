package web

import (
	"net/http"

	"aura-forge/internal/domain"
	"aura-forge/internal/service"
)

// TemplRenderer implements internal/http.Renderer using compiled a-h/templ components.
type TemplRenderer struct{}

func NewRenderer() *TemplRenderer {
	return &TemplRenderer{}
}

func (tr *TemplRenderer) RenderDashboard(w http.ResponseWriter, r *http.Request, data *service.DashboardSummary) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return DashboardPage(data).Render(r.Context(), w)
}

func (tr *TemplRenderer) RenderCatalog(w http.ResponseWriter, r *http.Request, mode service.ModeInfo, hub domain.TradeHub, items []domain.MarketItem, summary service.CatalogSummary, category, status, search, sortBy string, importRes *domain.ImportResult) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return CatalogPage(mode, hub, items, summary, category, status, search, sortBy, importRes).Render(r.Context(), w)
}

func (tr *TemplRenderer) RenderCatalogTable(w http.ResponseWriter, r *http.Request, hub domain.TradeHub, items []domain.MarketItem) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return CatalogTable(hub, items).Render(r.Context(), w)
}

func (tr *TemplRenderer) RenderCatalogRecommendations(w http.ResponseWriter, r *http.Request, res *service.CatalogRecommendResult) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return CatalogRecommendationsPartial(res).Render(r.Context(), w)
}

func (tr *TemplRenderer) RenderCalculator(w http.ResponseWriter, r *http.Request, mode service.ModeInfo, settings domain.UserSettings, input domain.ProfitCalcInput, result *domain.ProfitCalcResult, calcErr string, blueprints []domain.SDEBlueprint) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return CalculatorPage(mode, settings, input, result, calcErr, blueprints).Render(r.Context(), w)
}

func (tr *TemplRenderer) RenderCalculatorResult(w http.ResponseWriter, r *http.Request, input domain.ProfitCalcInput, result *domain.ProfitCalcResult, calcErr string) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return CalculatorResultPartial(input, result, calcErr).Render(r.Context(), w)
}

func (tr *TemplRenderer) RenderBlueprintResult(w http.ResponseWriter, r *http.Request, result *domain.BlueprintCalcResult, errMsg string) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return BlueprintResultPartial(result, errMsg).Render(r.Context(), w)
}

func (tr *TemplRenderer) RenderManufacture(w http.ResponseWriter, r *http.Request, mode service.ModeInfo, settings domain.UserSettings, blueprints []domain.SDEBlueprint, initialResult *domain.BlueprintCalcResult) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return ManufacturePage(mode, settings, blueprints, initialResult).Render(r.Context(), w)
}

func (tr *TemplRenderer) RenderManufactureTreePartial(w http.ResponseWriter, r *http.Request, result *domain.BlueprintCalcResult, errMsg string) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return ManufactureTreePartial(result, errMsg).Render(r.Context(), w)
}

func (tr *TemplRenderer) RenderAllocator(w http.ResponseWriter, r *http.Request, mode service.ModeInfo, cfg domain.AllocationConfig, result *domain.AllocationResult, allocErr string) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return AllocatorPage(mode, cfg, result, allocErr).Render(r.Context(), w)
}

func (tr *TemplRenderer) RenderAllocatorResult(w http.ResponseWriter, r *http.Request, result *domain.AllocationResult, allocErr string) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return AllocatorResultPartial(result, allocErr).Render(r.Context(), w)
}

func (tr *TemplRenderer) RenderPlanner(w http.ResponseWriter, r *http.Request, mode service.ModeInfo, data *service.PlannerViewData) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return PlannerPage(mode, data).Render(r.Context(), w)
}

func (tr *TemplRenderer) RenderTracker(w http.ResponseWriter, r *http.Request, mode service.ModeInfo, hub domain.TradeHub, activeTab string, summary service.TrackerSummary, batches []domain.BatchRecord, statusFilter string, ledger *service.TradeLedgerViewData) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return TrackerPage(mode, hub, activeTab, summary, batches, statusFilter, ledger).Render(r.Context(), w)
}

func (tr *TemplRenderer) RenderRelistResult(w http.ResponseWriter, r *http.Request, res *domain.RelistGuardResult, errMsg string) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return RelistResultPartial(res, errMsg).Render(r.Context(), w)
}

func (tr *TemplRenderer) RenderSettings(w http.ResponseWriter, r *http.Request, mode service.ModeInfo, settings domain.UserSettings, sdeStatus service.SDEImportStatus, pilotMem *domain.AIPilotMemory, saved bool) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return SettingsPage(mode, settings, sdeStatus, pilotMem, saved).Render(r.Context(), w)
}

func (tr *TemplRenderer) RenderSDEStatusPartial(w http.ResponseWriter, r *http.Request, status service.SDEImportStatus) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return SDEStatusPartial(status).Render(r.Context(), w)
}

func (tr *TemplRenderer) RenderAIDrawerThread(w http.ResponseWriter, r *http.Request, mode service.ModeInfo, convs []domain.AIConversation, activeConvID int64, msgs []domain.AIMessage, errMsg string) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return AIDrawerThreadPartial(mode, convs, activeConvID, msgs, errMsg).Render(r.Context(), w)
}

func (tr *TemplRenderer) RenderMineralTicker(w http.ResponseWriter, r *http.Request, pulse service.MarketPulseResponse) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return MineralTickerPartial(pulse).Render(r.Context(), w)
}

func (tr *TemplRenderer) RenderLogin(w http.ResponseWriter, r *http.Request, errMsg string, username string) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return LoginPage(errMsg, username).Render(r.Context(), w)
}

func (tr *TemplRenderer) RenderUsers(w http.ResponseWriter, r *http.Request, mode service.ModeInfo, users []domain.AppUser, flashMsg string, flashErr string) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return UsersPage(mode, users, flashMsg, flashErr).Render(r.Context(), w)
}

func (tr *TemplRenderer) RenderMarketplace(w http.ResponseWriter, r *http.Request, mode service.ModeInfo, query string, hubKey string, presets []service.MarketplacePreset, snap *domain.MarketplaceSnapshot, errMsg string) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return MarketplacePage(mode, query, hubKey, presets, snap, errMsg).Render(r.Context(), w)
}

func (tr *TemplRenderer) RenderMarketplaceResults(w http.ResponseWriter, r *http.Request, snap *domain.MarketplaceSnapshot, errMsg string) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return MarketplaceResultsPartial(snap, errMsg).Render(r.Context(), w)
}

func (tr *TemplRenderer) RenderMarketplaceAppraisal(w http.ResponseWriter, r *http.Request, res *domain.AppraisalResult, errMsg string) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return MarketplaceAppraisalPartial(res, errMsg).Render(r.Context(), w)
}

func (tr *TemplRenderer) RenderScanner(w http.ResponseWriter, r *http.Request, mode service.ModeInfo, settings domain.UserSettings) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return ScannerPage(mode, settings).Render(r.Context(), w)
}

func (tr *TemplRenderer) RenderScannerManufacturing(w http.ResponseWriter, r *http.Request, isAdmin bool, results []domain.ManufacturingOpportunity, errMsg string) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return ScannerManufacturingPartial(isAdmin, results, errMsg).Render(r.Context(), w)
}

func (tr *TemplRenderer) RenderScannerStationTrading(w http.ResponseWriter, r *http.Request, results []domain.StationTradeOpportunity, errMsg string) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return ScannerStationTradingPartial(results, errMsg).Render(r.Context(), w)
}

func (tr *TemplRenderer) RenderScannerArbitrage(w http.ResponseWriter, r *http.Request, results []domain.ArbitrageOpportunity, errMsg string) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return ScannerArbitragePartial(results, errMsg).Render(r.Context(), w)
}

func (tr *TemplRenderer) RenderScannerStockouts(w http.ResponseWriter, r *http.Request, results []domain.StockoutOpportunity, errMsg string) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return ScannerStockoutsPartial(results, errMsg).Render(r.Context(), w)
}

func (tr *TemplRenderer) RenderScannerReactions(w http.ResponseWriter, r *http.Request, results []domain.ReactionOpportunity, errMsg string) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return ScannerReactionsPartial(results, errMsg).Render(r.Context(), w)
}

func (tr *TemplRenderer) RenderDashboardRadar(w http.ResponseWriter, r *http.Request, snap service.RadarSnapshot) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return DashboardRadarPartial(snap).Render(r.Context(), w)
}

func (tr *TemplRenderer) RenderWealth(w http.ResponseWriter, r *http.Request, mode service.ModeInfo, summary *domain.WealthSummary) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return WealthPage(mode, summary).Render(r.Context(), w)
}

func (tr *TemplRenderer) RenderInvention(w http.ResponseWriter, r *http.Request, mode service.ModeInfo, settings domain.UserSettings) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return InventionPage(mode, settings).Render(r.Context(), w)
}

func (tr *TemplRenderer) RenderInventionResult(w http.ResponseWriter, r *http.Request, res *domain.InventionOptimizationResult, errMsg string) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return InventionResultPartial(res, errMsg).Render(r.Context(), w)
}

func (tr *TemplRenderer) RenderMakeVsBuyResult(w http.ResponseWriter, r *http.Request, res *domain.MakeVsBuyResult, errMsg string) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return MakeVsBuyResultPartial(res, errMsg).Render(r.Context(), w)
}

func (tr *TemplRenderer) RenderWiki(w http.ResponseWriter, r *http.Request, mode service.ModeInfo) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return WikiPage(mode).Render(r.Context(), w)
}

func (tr *TemplRenderer) RenderPilotHub(w http.ResponseWriter, r *http.Request, mode service.ModeInfo, view *service.PilotHubViewData) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return PilotHubPage(mode, view).Render(r.Context(), w)
}

func (tr *TemplRenderer) RenderPI(w http.ResponseWriter, r *http.Request, mode service.ModeInfo, items []domain.PICommodity, selectedTree *domain.PIRecipeAnalysis, colonies []domain.PIColony) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return PIPage(mode, items, selectedTree, colonies).Render(r.Context(), w)
}

func (tr *TemplRenderer) RenderPITreePartial(w http.ResponseWriter, r *http.Request, tree *domain.PIRecipeAnalysis, errMsg string) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return PITreePartial(tree, errMsg).Render(r.Context(), w)
}

func (tr *TemplRenderer) RenderPIScanPartial(w http.ResponseWriter, r *http.Request, rows []domain.PIProfitRow, tier string, hub domain.TradeHub, errMsg string) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return PIScanPartial(rows, tier, hub, errMsg).Render(r.Context(), w)
}

func (tr *TemplRenderer) RenderPISalesCalcPartial(w http.ResponseWriter, r *http.Request, res *domain.PISalesCalcResult, unmatched []string, errMsg string) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return PISalesCalcPartial(res, unmatched, errMsg).Render(r.Context(), w)
}
