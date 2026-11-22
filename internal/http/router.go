package http

import (
	"net/http"
	"os"
	"path/filepath"

	"aura-forge/internal/service"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter builds the go-chi router without session enforcement (for backward-compatible unit tests).
func NewRouter(svc *service.CopilotService, renderer Renderer) http.Handler {
	return NewRouterWithAuth(svc, nil, renderer)
}

// NewRouterWithAuth builds the go-chi router with all aura-forge routes, authentication, RBAC, and optional EVE SSO.
func NewRouterWithAuth(svc *service.CopilotService, authSvc *service.AuthService, renderer Renderer) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(SecurityHeadersMiddleware)
	r.Use(HTMXMiddleware)

	h := NewHandlers(svc, renderer)
	ssoAuth := NewAuthHandlers(svc)
	userAuth := NewUserHandlers(svc, authSvc, renderer)

	// Static files (public)
	staticDir := filepath.Join("web", "static")
	if _, err := os.Stat(staticDir); err == nil {
		fs := http.FileServer(http.Dir(staticDir))
		r.Handle("/static/*", http.StripPrefix("/static/", fs))
	}

	// Health check (public)
	r.Get("/health", h.HandleHealth)

	// Application Login (public — no self-registration)
	r.Get("/login", userAuth.HandleLoginPage)
	r.Post("/login", userAuth.HandleLoginSubmit)

	// Authenticated Application Routes (both 'user' and 'admin' roles)
	r.Group(func(r chi.Router) {
		r.Use(RequireAuth(authSvc))

		adminOnly := RequireAdminIfAuth(authSvc)

		r.Post("/logout", userAuth.HandleLogout)

		// Dashboard (Admin: Full Command Center | User: Live Market Pulse & Quick Calculator)
		r.Get("/", h.HandleDashboard)

		// Built-in EVE Online Wiki & UI Color Guide (accessible via Topbar)
		r.Get("/wiki", h.HandleWikiPage)

		// Live EVE Marketplace Browser (Public ESI — read/search for all, add-catalog for admin only)
		r.Route("/marketplace", func(r chi.Router) {
			r.Get("/", h.HandleMarketplace)
			r.Get("/search", h.HandleMarketplaceSearch)
			r.Get("/autocomplete", h.HandleMarketplaceAutocomplete)
			r.Get("/browse", h.HandleMarketplaceBrowse)
			r.Post("/appraise", h.HandleMarketplaceAppraise)
			r.With(adminOnly).Post("/add-catalog", h.HandleMarketplaceAddCatalog)
		})

		// Opportunity Scanner (Manufacturing ISK/hr, Station Trading, Inter-Hub Arbitrage)
		r.Route("/scanner", func(r chi.Router) {
			r.Get("/", h.HandleScannerPage)
			r.Post("/manufacturing", h.HandleScanManufacturing)
			r.Post("/station-trading", h.HandleScanStationTrading)
			r.Post("/arbitrage", h.HandleScanArbitrage)
		})

		// Public ESI actions (lookup & market pulse for all; sync/undercuts for admin only)
		r.Route("/esi", func(r chi.Router) {
			r.Get("/lookup", h.HandleLookupItemESI)
			r.Get("/minerals", h.HandleLiveMinerals)
			r.With(adminOnly).Post("/sync-prices", h.HandleSyncJitaPrices)
			r.With(adminOnly).Post("/sync-sci", h.HandleSyncSCI)
			r.With(adminOnly).Post("/check-undercuts", h.HandleCheckUndercuts)
		})

		// Net Profit, Tax & Break-Even Calculator (Manual Mode + Blueprint SDE Mode; save-catalog for admin only)
		r.Route("/calculator", func(r chi.Router) {
			r.Get("/", h.HandleCalculator)
			r.Post("/compute", h.HandleCalculatorCompute)
			r.Get("/blueprints/search", h.HandleBlueprintSearch)
			r.Post("/blueprint/compute", h.HandleBlueprintCompute)
			r.With(adminOnly).Post("/blueprint/save-catalog", h.HandleBlueprintSaveCatalog)
		})

		// T2 Invention & Decryptor Optimizer + Make-vs-Buy Component Analyzer
		r.Route("/invention", func(r chi.Router) {
			r.Get("/", h.HandleInventionPage)
			r.Post("/optimize", h.HandleInventionOptimize)
			r.Post("/make-vs-buy", h.HandleMakeVsBuy)
		})

		// Manufacture Horizontal Blueprint Recipe Tree & Cost Explorer
		r.Route("/manufacture", func(r chi.Router) {
			r.Get("/", h.HandleManufacturePage)
			r.Post("/tree", h.HandleManufactureTree)
		})

		// Aura AI Level-4 Agentic Copilot (Optional OpenAI-Compatible API)
		r.Route("/ai", func(r chi.Router) {
			r.Get("/drawer", h.HandleAIDrawer)
			r.Post("/chat", h.HandleAIChat)
			r.Delete("/conversations/{id}", h.HandleAIDeleteConversation)
			r.Post("/memory", h.HandleAISaveMemory)
			r.With(adminOnly).Post("/test", h.HandleAITestConnection)
		})

		// Planetary Industry (PI) Hub (Research tabs for all; My Colonies mutations for admin only)
		r.Route("/pi", func(r chi.Router) {
			r.Get("/", h.HandlePIPage)
			r.Get("/tree", h.HandlePITree)
			r.Post("/scan", h.HandlePIScan)
			r.Post("/sales-calc", h.HandlePISalesCalc)
			r.With(adminOnly).Post("/colonies", h.HandlePIColonySave)
			r.With(adminOnly).Post("/colonies/{id}/reset", h.HandlePIColonyReset)
			r.With(adminOnly).Delete("/colonies/{id}", h.HandlePIColonyDelete)
		})

		// Admin-Only Business, Financial & System Routes
		r.Group(func(r chi.Router) {
			r.Use(adminOnly)

			// Catalog & Bulk Importer
			r.Route("/catalog", func(r chi.Router) {
				r.Get("/", h.HandleCatalog)
				r.Post("/hub", h.HandleSetCatalogHub)
				r.Post("/items", h.HandleSaveCatalogItem)
				r.Delete("/items/{id}", h.HandleDeleteCatalogItem)
				r.Post("/recommend", h.HandleCatalogRecommend)
				r.Post("/recommend/apply", h.HandleCatalogRecommendApply)
				r.Post("/import", h.HandleBulkImport)
				r.Post("/sample", h.HandleLoadSampleData)
			})

			// Smart Capital Allocator
			r.Route("/allocator", func(r chi.Router) {
				r.Get("/", h.HandleAllocator)
				r.Post("/run", h.HandleAllocatorRun)
				r.Post("/to-plan", h.HandleAllocatorToPlan)
				r.Post("/to-batches", h.HandleAllocatorToBatches)
			})

			// Net Worth Tracker (0 -> 100B+ Milestone Curve) & Capital-Phase Advisor
			r.Route("/wealth", func(r chi.Router) {
				r.Get("/", h.HandleWealthPage)
				r.Post("/snapshot", h.HandleSaveWealthSnapshot)
				r.Post("/auto-snapshot", h.HandleAutoWealthSnapshot)
				r.Delete("/snapshot/{id}", h.HandleDeleteWealthSnapshot)
			})

			// Weekly Production & Slot Planner
			r.Route("/planner", func(r chi.Router) {
				r.Get("/", h.HandlePlanner)
				r.Post("/convert-batches", h.HandlePlannerConvertBatches)
			})

			// Production, Inventory & Market Order Tracker (+ Buy/Sell Ledger & Realized P&L)
			r.Route("/tracker", func(r chi.Router) {
				r.Get("/", h.HandleTracker)
				r.Post("/batches", h.HandleSaveBatch)
				r.Post("/batches/{id}/status", h.HandleUpdateBatchStatus)
				r.Delete("/batches/{id}", h.HandleDeleteBatch)
				r.Post("/batches/{id}/relist-check", h.HandleRelistCheck)
				r.Post("/ledger/sync-esi", h.HandleLedgerSyncESI)
				r.Post("/ledger/import-paste", h.HandleLedgerImportPaste)
				r.Post("/ledger/manual", h.HandleLedgerAddManual)
				r.Post("/ledger/exclude", h.HandleLedgerSetExcluded)
				r.Delete("/ledger/tx/{id}", h.HandleLedgerDeleteTx)
			})

			// Settings, Official CCP SDE Importer & Background Price Updater
			r.Route("/settings", func(r chi.Router) {
				r.Get("/", h.HandleSettings)
				r.Post("/", h.HandleSaveSettings)
				r.Post("/sde/import", h.HandleSDEImport)
				r.Get("/sde/status", h.HandleSDEStatus)
			})

			// Optional EVE SSO OAuth2 Routes (Admin only)
			r.Route("/auth", func(r chi.Router) {
				r.Get("/login", ssoAuth.HandleLogin)
				r.Get("/callback", ssoAuth.HandleCallback)
				r.Post("/disconnect", ssoAuth.HandleDisconnect)
				r.Post("/sync", ssoAuth.HandleSync)
			})
		})

		// Admin-Only User Management Routes (strictly RequireAdmin)
		r.Route("/admin/users", func(r chi.Router) {
			r.Use(RequireAdmin)
			r.Get("/", userAuth.HandleUsersPage)
			r.Post("/", userAuth.HandleCreateUser)
			r.Post("/{id}/update", userAuth.HandleUpdateUser)
			r.Post("/{id}/reset-password", userAuth.HandleResetUserPassword)
			r.Delete("/{id}", userAuth.HandleDeleteUser)
		})
	})

	return r
}
