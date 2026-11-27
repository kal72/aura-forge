package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"aura-forge/internal/ai"
	"aura-forge/internal/config"
	"aura-forge/internal/domain"
	"aura-forge/internal/esi"
	apphttp "aura-forge/internal/http"
	"aura-forge/internal/service"
	"aura-forge/internal/store/postgres"
	"aura-forge/internal/web"
)

func main() {
	cfg := config.Load()

	fmt.Println("================================================================")
	fmt.Println("  aura-forge // EVE Market & Industry Copilot (2026)")
	fmt.Println("================================================================")
	fmt.Printf("Port            : %s\n", cfg.Port)
	fmt.Printf("Database URL    : %s\n", cfg.DatabaseURL)
	fmt.Printf("Public ESI Agent: %s\n", cfg.ESIUserAgent)
	if cfg.IsSSOEnabled() {
		fmt.Println("EVE SSO Status  : ENABLED (Authenticated Character Sync Active)")
	} else {
		fmt.Println("EVE SSO Status  : DISABLED (Standard Mode: Public ESI + Manual/Import)")
	}
	if cfg.IsAIEnabled() {
		fmt.Printf("Aura AI Status  : ENABLED (Model: %s @ %s)\n", cfg.AIModel, cfg.AIBaseURL)
	} else {
		fmt.Println("Aura AI Status  : DISABLED (Optional: Set AI_API_KEY in .env for Level 4 Copilot)")
	}
	fmt.Println("================================================================")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var (
		itemRepo     domain.ItemRepository
		batchRepo    domain.BatchRepository
		planRepo     domain.PlanRepository
		charRepo     domain.CharacterRepository
		settingsRepo domain.SettingsRepository
		cacheRepo    domain.ESICacheRepository
		userRepo     domain.UserRepository
		sdeRepo            domain.SDERepository
		wealthRepo         domain.NetWorthRepository
		piRepo             domain.PIRepository
		globalSnapshotRepo domain.GlobalPriceSnapshotRepository
		aiRepo             domain.AIRepository
		ledgerRepo         domain.TradeLedgerRepository
		tacticalRepo       domain.TacticalRepository
	)

	db, err := postgres.NewDB(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("[FATAL] PostgreSQL connection or migration failed: %v (refusing to start in in-memory mode)", err)
	}
	defer db.Close()
	log.Println("[INFO] Connected to PostgreSQL and migrations verified.")
	itemRepo = postgres.NewItemRepo(db.Pool)
	batchRepo = postgres.NewBatchRepo(db.Pool)
	planRepo = postgres.NewPlanRepo(db.Pool)
	charRepo = postgres.NewCharacterRepo(db.Pool)
	settingsRepo = postgres.NewSettingsRepo(db.Pool)
	cacheRepo = postgres.NewCacheRepo(db.Pool)
	userRepo = postgres.NewUserRepo(db.Pool)
	sdeRepo = postgres.NewSDERepo(db.Pool)
	wealthRepo = postgres.NewWealthRepo(db.Pool)
	piRepo = postgres.NewPIRepo(db.Pool)
	globalSnapshotRepo = postgres.NewMarketSnapshotRepo(db.Pool)
	aiRepo = postgres.NewAIRepo(db.Pool)
	ledgerRepo = postgres.NewLedgerRepo(db.Pool)
	tacticalRepo = postgres.NewTacticalRepo(db.Pool)

	// Initialize Application Auth & seed initial Admin account if database has no users yet
	authSvc := service.NewAuthService(userRepo)
	if created, seedErr := authSvc.EnsureDefaultAdmin(context.Background(), cfg.AdminUsername, cfg.AdminPassword); seedErr != nil {
		log.Printf("[WARN] Failed seeding default admin account: %v", seedErr)
	} else if created {
		log.Printf("[INFO] Initial Admin account created (username=%q). Change password in User Management.", cfg.AdminUsername)
	}

	// Initialize ESI adapters (Public ESI always active; SSO optional)
	esiClient := esi.NewClient(esi.DefaultESIBaseURL, cfg.ESIUserAgent, cacheRepo, nil)
	publicMarket := esi.NewPublicMarketClient(esiClient)
	ssoClient := esi.NewSSOClient(cfg.ESIClientID, cfg.ESIClientSecret, cfg.ESICallbackURL, nil)

	var charSync domain.CharacterSyncProvider
	if cfg.IsSSOEnabled() {
		charSync = esi.NewCharacterSyncClient(esiClient, ssoClient, charRepo)
	} else {
		charSync = esi.NewNoOpCharacterSyncProvider()
	}

	// Wire Clean Architecture service & router
	copilotSvc := service.NewCopilotService(
		itemRepo,
		batchRepo,
		planRepo,
		charRepo,
		settingsRepo,
		publicMarket,
		charSync,
		ssoClient,
	)
	copilotSvc.SetSDERepository(sdeRepo)
	copilotSvc.SetNetWorthRepository(wealthRepo)
	copilotSvc.SetPIRepository(piRepo)
	copilotSvc.SetGlobalPriceSnapshotRepository(globalSnapshotRepo)
	copilotSvc.SetAIRepository(aiRepo)
	copilotSvc.SetTradeLedgerRepository(ledgerRepo)
	copilotSvc.SetTacticalRepository(tacticalRepo)
	aiClient := ai.NewOpenAIClient(cfg.AIBaseURL, cfg.AIAPIKey, cfg.AIModel, time.Duration(cfg.AITimeoutSeconds)*time.Second)
	copilotSvc.SetAIProvider(aiClient)

	// Start supervised background Jita price updater (Catalog + PI P1-P4 + SDE Blueprints)
	bgCtx, bgCancel := context.WithCancel(context.Background())
	defer bgCancel()
	copilotSvc.StartBackgroundPriceUpdater(bgCtx)
	copilotSvc.StartBackgroundOpportunityRadar(bgCtx)

	router := apphttp.NewRouterWithAuth(copilotSvc, authSvc, web.NewRenderer())

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("[INFO] Server listening on http://localhost:%s\n", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[INFO] Shutting down aura-forge server...")
	bgCancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
	log.Println("[INFO] Server stopped gracefully.")
}
