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

		r.Post("/logout", userAuth.HandleLogout)

		// Dashboard
		r.Get("/", h.HandleDashboard)

		// Live EVE Marketplace Browser (Public ESI)
		r.Route("/marketplace", func(r chi.Router) {
			r.Get("/", h.HandleMarketplace)
			r.Get("/search", h.HandleMarketplaceSearch)
			r.Post("/add-catalog", h.HandleMarketplaceAddCatalog)
		})

		// Catalog & Bulk Importer
		r.Route("/catalog", func(r chi.Router) {
			r.Get("/", h.HandleCatalog)
			r.Post("/items", h.HandleSaveCatalogItem)
			r.Delete("/items/{id}", h.HandleDeleteCatalogItem)
			r.Post("/import", h.HandleBulkImport)
			r.Post("/sample", h.HandleLoadSampleData)
		})

		// Public ESI actions
		r.Route("/esi", func(r chi.Router) {
			r.Post("/sync-prices", h.HandleSyncJitaPrices)
			r.Post("/sync-sci", h.HandleSyncSCI)
			r.Get("/lookup", h.HandleLookupItemESI)
			r.Get("/minerals", h.HandleLiveMinerals)
			r.Post("/check-undercuts", h.HandleCheckUndercuts)
		})

		// Net Profit, Tax & Break-Even Calculator
		r.Route("/calculator", func(r chi.Router) {
			r.Get("/", h.HandleCalculator)
			r.Post("/compute", h.HandleCalculatorCompute)
		})

		// Smart Capital Allocator
		r.Route("/allocator", func(r chi.Router) {
			r.Get("/", h.HandleAllocator)
			r.Post("/run", h.HandleAllocatorRun)
			r.Post("/to-plan", h.HandleAllocatorToPlan)
			r.Post("/to-batches", h.HandleAllocatorToBatches)
		})

		// Weekly Production & Slot Planner
		r.Route("/planner", func(r chi.Router) {
			r.Get("/", h.HandlePlanner)
			r.Post("/convert-batches", h.HandlePlannerConvertBatches)
		})

		// Production, Inventory & Market Order Tracker
		r.Route("/tracker", func(r chi.Router) {
			r.Get("/", h.HandleTracker)
			r.Post("/batches", h.HandleSaveBatch)
			r.Post("/batches/{id}/status", h.HandleUpdateBatchStatus)
			r.Delete("/batches/{id}", h.HandleDeleteBatch)
			r.Post("/batches/{id}/relist-check", h.HandleRelistCheck)
		})

		// Settings
		r.Route("/settings", func(r chi.Router) {
			r.Get("/", h.HandleSettings)
			r.Post("/", h.HandleSaveSettings)
		})

		// Optional EVE SSO OAuth2 Routes (available after login)
		r.Route("/auth", func(r chi.Router) {
			r.Get("/login", ssoAuth.HandleLogin)
			r.Get("/callback", ssoAuth.HandleCallback)
			r.Post("/disconnect", ssoAuth.HandleDisconnect)
			r.Post("/sync", ssoAuth.HandleSync)
		})

		// Admin-Only User Management Routes
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
