package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	// Backup env and restore after test
	oldListen := os.Getenv("LISTEN_ADDR")
	oldDB := os.Getenv("DB_PATH")
	defer func() {
		_ = os.Setenv("LISTEN_ADDR", oldListen)
		_ = os.Setenv("DB_PATH", oldDB)
	}()

	t.Run("Defaults", func(t *testing.T) {
		_ = os.Unsetenv("LISTEN_ADDR")
		_ = os.Unsetenv("DB_PATH")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if cfg.ListenAddr != ":9096" {
			t.Errorf("Expected default ListenAddr :9096, got %s", cfg.ListenAddr)
		}
		if cfg.DBPath != "clusteruptime.db" {
			t.Errorf("Expected default DBPath clusteruptime.db, got %s", cfg.DBPath)
		}
	})

	t.Run("Env Overrides", func(t *testing.T) {
		_ = os.Setenv("LISTEN_ADDR", ":8080")
		_ = os.Setenv("DB_PATH", "/tmp/test.db")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if cfg.ListenAddr != ":8080" {
			t.Errorf("Expected ListenAddr :8080, got %s", cfg.ListenAddr)
		}
		if cfg.DBPath != "/tmp/test.db" {
			t.Errorf("Expected DBPath /tmp/test.db, got %s", cfg.DBPath)
		}
	})
}
