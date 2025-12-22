package db

import (
	"testing"
)

func newTestStore(t *testing.T) *Store {
	store, err := NewStore(":memory:")
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
