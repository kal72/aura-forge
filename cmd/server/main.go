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
	"aura-forge/internal/store/postgres"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
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

	// Context for startup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Initialize DB (optional during dry-run build or connects if running)
	db, err := postgres.NewDB(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Printf("[WARN] PostgreSQL not reachable at %s: %v", cfg.DatabaseURL, err)
		log.Println("[INFO] Proceeding without active database connection for dry-run/bootstrap.")
	} else {
		defer db.Close()
		log.Println("[INFO] Connected to PostgreSQL and migrations verified.")
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","app":"aura-forge","mode":"copilot"}`))
	})

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
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
