package db

import (
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
