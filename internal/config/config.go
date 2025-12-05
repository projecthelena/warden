package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	ListenAddr    string
	CheckInterval time.Duration
	TargetURL     string
}

func Default() Config {
	return Config{
		ListenAddr:    ":9090",
		CheckInterval: 10 * time.Second,
		TargetURL:     "",
	}
}

func Load() (Config, error) {
	cfg := Default()

	if listen := os.Getenv("LISTEN_ADDR"); listen != "" {
		cfg.ListenAddr = listen
	}

	if interval := os.Getenv("CHECK_INTERVAL"); interval != "" {
		d, err := time.ParseDuration(interval)
		if err != nil {
			return Config{}, fmt.Errorf("invalid CHECK_INTERVAL: %w", err)
		}
		cfg.CheckInterval = d
	}

	if target := os.Getenv("TARGET_URL"); target != "" {
		cfg.TargetURL = target
	}

	if cfg.TargetURL == "" {
		// Fallback for development/testing if mostly needed, or error
		// For now let's default to something if empty or just return default
		// But user asked for uptime status to *some api*.
		// Let's require it or default to google.com for demo
		if os.Getenv("ENV") == "dev" {
			cfg.TargetURL = "https://www.google.com"
		}
	}

	return cfg, nil
}
