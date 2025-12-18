package config

import (
	"os"
)

type Config struct {
	ListenAddr   string
	DBPath       string
	CookieSecure bool
}

func Default() Config {
	return Config{
		ListenAddr:   ":9096",
		DBPath:       "clusteruptime.db",
		CookieSecure: false,
	}
}

func Load() (*Config, error) {
	cfg := Default()

	if listen := os.Getenv("LISTEN_ADDR"); listen != "" {
		cfg.ListenAddr = listen
	}

	if dbPath := os.Getenv("DB_PATH"); dbPath != "" {
		cfg.DBPath = dbPath
	}

	if os.Getenv("COOKIE_SECURE") == "true" {
		cfg.CookieSecure = true
	}

	return &cfg, nil
}
