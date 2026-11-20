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

	"aura-forge/internal/config"
	"aura-forge/internal/domain"
	"aura-forge/internal/esi"
	apphttp "aura-forge/internal/http"
	"aura-forge/internal/service"
	"aura-forge/internal/store/postgres"
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
	)

	db, err := postgres.NewDB(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Printf("[WARN] PostgreSQL not reachable at %s: %v", cfg.DatabaseURL, err)
		log.Println("[INFO] Run 'make db-up' to start PostgreSQL 16. Proceeding in degraded mode.")
	} else {
		defer db.Close()
		log.Println("[INFO] Connected to PostgreSQL and migrations verified.")
		itemRepo = postgres.NewItemRepo(db.Pool)
		batchRepo = postgres.NewBatchRepo(db.Pool)
		planRepo = postgres.NewPlanRepo(db.Pool)
		charRepo = postgres.NewCharacterRepo(db.Pool)
		settingsRepo = postgres.NewSettingsRepo(db.Pool)
		cacheRepo = postgres.NewCacheRepo(db.Pool)
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

	router := apphttp.NewRouter(copilotSvc, nil)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
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
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
	log.Println("[INFO] Server stopped gracefully.")
}
