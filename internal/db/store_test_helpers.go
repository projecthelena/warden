package db

import (
	"os"
	"testing"
)

// NewTestConfig returns a DBConfig for in-memory SQLite testing
func NewTestConfig() DBConfig {
	return DBConfig{
		Type: DialectSQLite,
		Path: ":memory:",
	}
}

// NewTestConfigWithPath returns a DBConfig for SQLite testing with a specific path
func NewTestConfigWithPath(path string) DBConfig {
	return DBConfig{
		Type: DialectSQLite,
		Path: path,
	}
}

// NewPostgresTestConfig returns a DBConfig for PostgreSQL testing
// Returns nil if TEST_POSTGRES_URL environment variable is not set
func NewPostgresTestConfig() *DBConfig {
	url := os.Getenv("TEST_POSTGRES_URL")
	if url == "" {
		return nil
	}
	return &DBConfig{
		Type: DialectPostgres,
		URL:  url,
	}
}

func newTestStore(t *testing.T) *Store {
	store, err := NewStore(NewTestConfig())
	if err != nil {
		t.Fatalf("Failed to create test store: %v", err)
	}
	// NewStore calls migrate() implementation.
	// We rely on that.

	// Clear seeded data (e.g. g-default group) to ensure clean tests
	_, _ = store.db.Exec("DELETE FROM groups")
	_, _ = store.db.Exec("DELETE FROM monitors")

	return store
}

// TestDBConfig represents a database configuration for testing
type TestDBConfig struct {
	Name   string
	Config DBConfig
}

// GetTestConfigs returns all available database configurations for testing
// Always includes SQLite, includes PostgreSQL if TEST_POSTGRES_URL is set
func GetTestConfigs(t *testing.T) []TestDBConfig {
	configs := []TestDBConfig{
		{
			Name:   "SQLite",
			Config: NewTestConfig(),
		},
	}

	if pgConfig := NewPostgresTestConfig(); pgConfig != nil {
		configs = append(configs, TestDBConfig{
			Name:   "PostgreSQL",
			Config: *pgConfig,
		})
	}

	return configs
}

// RunTestWithBothDBs runs a test function against all available database backends
// Use this for tests that should verify behavior on both SQLite and PostgreSQL
func RunTestWithBothDBs(t *testing.T, name string, testFn func(t *testing.T, store *Store)) {
	configs := GetTestConfigs(t)

	for _, cfg := range configs {
		t.Run(cfg.Name, func(t *testing.T) {
			store, err := NewStore(cfg.Config)
			if err != nil {
				t.Fatalf("Failed to create %s store: %v", cfg.Name, err)
			}
			defer func() { _ = store.Close() }()

			// Clear seeded data for clean tests
			_, _ = store.db.Exec("DELETE FROM monitors")
			_, _ = store.db.Exec("DELETE FROM groups")

			testFn(t, store)

			// Clean up after test
			if cfg.Config.Type == DialectPostgres {
				// Reset the database for PostgreSQL to avoid test pollution
				_ = store.Reset()
			}
		})
	}
}
