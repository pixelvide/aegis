// Package main is the entry point for the Aegis server.
//
// The server provides:
// - REST API for managing findings and exploits
// - Agent Ingest API for external AI agents to push findings
// - API token management for agent authentication
// - Static file serving for the React UI
// - Multi-tenant architecture: common schema + per-org schemas
//
// Security: Binds to 127.0.0.1 by default. Set AEGIS_BIND=0.0.0.0 for Docker.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pixelvide/aegis/server/internal/api"
	"github.com/pixelvide/aegis/server/internal/auth"
	"github.com/pixelvide/aegis/server/internal/cache"
	"github.com/pixelvide/aegis/server/internal/config"
	"github.com/pixelvide/aegis/server/internal/email"
	"github.com/pixelvide/aegis/server/internal/logger"
	"github.com/pixelvide/aegis/server/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server fatal error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	// Initialize structured logger first — all subsequent code uses slog
	logger.Init(cfg.LogLevel, cfg.LogFormat)
	slog.Info("configuration loaded", "log_level", cfg.LogLevel, "log_format", cfg.LogFormat)

	// Open PostgreSQL connection pool
	if cfg.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required (e.g. postgres://aegis:aegis@localhost:5432/aegis?sslmode=disable)")
	}

	db, err := store.NewPostgres(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	defer db.Close()
	slog.Info("connected to PostgreSQL", "component", "database")

	// Initialize common schema (users, orgs, memberships)
	common, err := store.NewCommonStore(db)
	if err != nil {
		return fmt.Errorf("common store: %w", err)
	}
	slog.Info("common schema ready", "component", "store")

	// Initialize auth service (JWT + password hashing)
	authSvc, err := auth.New()
	if err != nil {
		return fmt.Errorf("auth: %w", err)
	}
	slog.Info("auth service ready", "component", "auth")

	// Initialize email service (SMTP)
	emailSvc := email.New(cfg.SMTP)

	// Initialize Valkey cache (optional — graceful degradation to DB-only)
	var cacheClient *cache.Client
	if cfg.ValkeyURL != "" {
		cacheClient, err = cache.New(cfg.ValkeyURL)
		if err != nil {
			slog.Warn("valkey unavailable, falling back to DB-only session checks",
				"error", err, "component", "cache")
			cacheClient = nil
		} else {
			defer cacheClient.Close()
			slog.Info("valkey connected", "addr", cfg.ValkeyURL, "component", "cache")
		}
	} else {
		slog.Info("valkey not configured, using DB-only session checks", "component", "cache")
	}

	// Initialize OTel metrics with Prometheus exporter
	metrics, metricsHandler, metricsShutdown := api.InitMetrics(db)
	defer metricsShutdown(context.Background())
	slog.Info("metrics ready", "endpoint", "/metrics", "component", "otel")

	// Create API server
	apiSrv := api.New(common, authSvc, emailSvc, cacheClient, cfg)

	// Health check handler
	health := api.NewHealthHandler(db)

	// Combined handler: API + health + metrics + UI
	mux := http.NewServeMux()
	mux.Handle("/api/", apiSrv)
	mux.HandleFunc("GET /healthz", health.HandleHealthz)
	mux.HandleFunc("GET /readyz", health.HandleReadyz)
	mux.Handle("GET /metrics", metricsHandler)
	mux.Handle("/", uiHandler())

	// Wrap with auth page redirect (302 for /login etc. on non-base-domain)
	// then metrics middleware
	handler := metrics.Middleware(api.AuthPageRedirect(cfg)(mux))

	// HTTP server
	httpServer := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1MB
	}

	// Graceful shutdown
	errCh := make(chan error, 1)
	go func() {
		slog.Info("server listening", "addr", cfg.Addr())
		errCh <- httpServer.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		slog.Info("received signal, shutting down", "signal", sig.String())
	case err := <-errCh:
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return httpServer.Shutdown(ctx)
}
