package main

import (
	"context"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/clusteruptime/clusteruptime/internal/api"
	"github.com/clusteruptime/clusteruptime/internal/config"
	"github.com/clusteruptime/clusteruptime/internal/logging"
	"github.com/clusteruptime/clusteruptime/internal/uptime"
)

func main() {
	logger := logging.New("clusteruptime")

	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("load config: %v", err)
	}

	monitor := uptime.NewMonitor(cfg)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start Monitor in background
	go monitor.Start(ctx)

	srv := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: api.NewRouter(monitor),
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	logger.Printf("listening on %s", cfg.ListenAddr)
	if cfg.TargetURL != "" {
		logger.Printf("monitoring target: %s", cfg.TargetURL)
	} else {
		logger.Printf("no target url configured (set TARGET_URL)")
	}

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("server error: %v", err)
	}
}
