package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/clusteruptime/clusteruptime/internal/api"
	"github.com/clusteruptime/clusteruptime/internal/config"
	"github.com/clusteruptime/clusteruptime/internal/db"
	"github.com/clusteruptime/clusteruptime/internal/logging"
	"github.com/clusteruptime/clusteruptime/internal/uptime"
)

func main() {
	logger := logging.New("clusteruptime")

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
	store, err := db.NewStore(cfg.DBPath) // Changed "clusteruptime.db" to cfg.DBPath
	if err != nil {
		log.Fatal("Failed to init database:", err) // Changed logger.Fatalf to log.Fatal
	}
	defer func() { _ = store.Close() }()

	// Init Uptime Manager
	manager := uptime.NewManager(store)
	manager.Start()
	defer manager.Stop()

	// Init Router
	r := api.NewRouter(manager, store, cfg) // Changed monitor to manager

	srv := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: r,
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

	log.Println("Server exiting")
}
