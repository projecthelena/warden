package main

import (
	"context"
	"log"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/projecthelena/warden/internal/api"
	"github.com/projecthelena/warden/internal/config"
	"github.com/projecthelena/warden/internal/db"
	"github.com/projecthelena/warden/internal/logging"
	"github.com/projecthelena/warden/internal/observability"
	"github.com/projecthelena/warden/internal/uptime"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// @title           Warden API
// @version         1.0
// @description     Self-hosted uptime monitoring API by Project Helena.
// @BasePath        /api
// @securityDefinitions.apikey BearerAuth
// @in              header
// @name            Authorization
// @description     Enter "Bearer sk_live_..." — create keys in Settings > API Keys
func main() {
	// Subcommands run against the same database and exit without starting the server.
	if len(os.Args) > 1 && os.Args[1] == "reset-password" {
		os.Exit(runResetPassword(os.Args[2:]))
	}

	logger := logging.New("warden")

	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("load config: %v", err)
	}

	// monitor := uptime.NewMonitor(cfg) // Removed

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start Monitor in background // This comment is now misleading, as monitor is removed.
	// go monitor.Start(ctx) // Removed

	// Init DB
	store, err := db.NewStore(db.DBConfig{
		Type: cfg.DBType,
		Path: cfg.DBPath,
		URL:  cfg.DBURL,
	})
	if err != nil {
		log.Fatal("Failed to init database:", err)
	}
	log.Printf("Database initialized (dialect: %s)", store.Dialect())
	observability.RegisterDatabaseStats(store.Dialect(), store.Stats)
	defer func() { _ = store.Close() }()

	// Init Uptime Manager
	manager := uptime.NewManager(store)
	manager.Start()
	defer manager.Stop()

	var observabilityServer *http.Server
	if cfg.ObservabilityAddr != "" {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
		observabilityServer = &http.Server{
			Addr:              cfg.ObservabilityAddr,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
		}
		go func() {
			log.Printf("Starting observability server on %s", cfg.ObservabilityAddr)
			if err := observabilityServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Printf("observability server: %v", err)
			}
		}()
	}

	// Init Router
	r := api.NewRouter(manager, store, cfg) // Changed monitor to manager

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second, // Prevent Slowloris attacks
	}

	go func() {
		log.Printf("Starting server on %s", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// Wait for interrupt signal
	<-ctx.Done()
	log.Println("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	if observabilityServer != nil {
		if err := observabilityServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("observability server shutdown: %v", err)
		}
	}

	log.Println("Server exiting")
}
