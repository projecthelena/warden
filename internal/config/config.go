package config

import (
	"os"
	"strings"
)

// Database types
const (
	DBTypeSQLite   = "sqlite"
	DBTypePostgres = "postgres"
)

type Config struct {
	ListenAddr   string
	DBType       string // "sqlite" or "postgres"
	DBPath       string // SQLite file path (only used when DBType is "sqlite")
	DBURL        string // PostgreSQL connection URL (only used when DBType is "postgres")
	CookieSecure bool
	AdminSecret  string
	TrustProxy   bool // Trust X-Forwarded-For headers (only enable behind a trusted reverse proxy)
}

func Default() Config {
	return Config{
		ListenAddr:   ":9096",
		DBType:       DBTypeSQLite,
		DBPath:       "warden.db",
		CookieSecure: false,
	}
}

func Load() (*Config, error) {
	cfg := Default()

	if listen := os.Getenv("LISTEN_ADDR"); listen != "" {
		cfg.ListenAddr = listen
	}

	// Database configuration
	// DB_TYPE: "sqlite" (default) or "postgres"
	if dbType := os.Getenv("DB_TYPE"); dbType != "" {
		cfg.DBType = strings.ToLower(dbType)
	}

	// DB_PATH: SQLite file path (only used for sqlite)
	if dbPath := os.Getenv("DB_PATH"); dbPath != "" {
		cfg.DBPath = dbPath
	}

	// DB_URL: PostgreSQL connection string (only used for postgres)
	// Example: postgres://user:password@localhost:5432/warden?sslmode=disable
	if dbURL := os.Getenv("DB_URL"); dbURL != "" {
		cfg.DBURL = dbURL
		// Auto-detect postgres from URL if DB_TYPE not explicitly set
		if os.Getenv("DB_TYPE") == "" && strings.HasPrefix(dbURL, "postgres") {
			cfg.DBType = DBTypePostgres
		}
	}

	if os.Getenv("COOKIE_SECURE") == "true" {
		cfg.CookieSecure = true
	}

	if secret := os.Getenv("ADMIN_SECRET"); secret != "" {
		cfg.AdminSecret = secret
	}

	// TRUST_PROXY: Enable only when running behind a trusted reverse proxy (nginx, Traefik, etc.)
	// SECURITY WARNING: If enabled without a trusted proxy, attackers can spoof their IP address
	// via X-Forwarded-For headers, bypassing rate limiting and IP-based security controls.
	// Leave disabled (default) when exposing the server directly to the internet.
	if os.Getenv("TRUST_PROXY") == "true" {
		cfg.TrustProxy = true
	}

	return &cfg, nil
}
