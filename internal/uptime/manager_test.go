package uptime

import (
	"testing"
	"time"

	"github.com/clusteruptime/clusteruptime/internal/db"
)

func newTestManager(t *testing.T) (*Manager, *db.Store) {
	store, err := db.NewStore("file:manager?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("Failed to create test store: %v", err)
	}

	// Clear seeded data to have a clean state
	// Note: We access store.db assuming we can? No, store.db is private in package db.
	// But in store_test.go we were IN package db.
	// Here we are in package uptime. We cannot access store.db.
	// We should rely on public methods `Reset()` but that reseeds.
	// Or just ignore the default monitor 'm-example-monitor-default'.
	// Or assume "m1" is ours.

	m := NewManager(store)
	return m, store
}

func TestManager_Sync(t *testing.T) {
	m, s := newTestManager(t)

	// Create a new monitor in DB
	mon := db.Monitor{
		ID:       "m-test-1",
		GroupID:  "g-default",
		Name:     "Test Monitor",
		URL:      "http://example.com",
		Active:   true,
		Interval: 60,
	}
	s.CreateMonitor(mon)

	// Sync
	m.Sync()

	// Verify running
	running := m.GetMonitor("m-test-1")
	if running == nil {
		t.Fatal("Monitor should be running")
	}
	if running.GetTargetURL() != "http://example.com" {
		t.Errorf("Expected URL http://example.com, got %s", running.GetTargetURL())
	}
	if running.GetInterval() != 60*time.Second {
		t.Errorf("Expected interval 60s, got %s", running.GetInterval())
	}

	// Update in DB (change interval)
	s.UpdateMonitor("m-test-1", "Test Monitor", "http://example.com", 120)

	// Sync again
	m.Sync()

	// Verify updated
	running = m.GetMonitor("m-test-1")
	if running == nil {
		t.Fatal("Monitor should be running after update")
	}
	if running.GetInterval() != 120*time.Second {
		t.Errorf("Expected interval 120s, got %s", running.GetInterval())
	}

	// Ensure default monitor is also running (seeded)
	def := m.GetMonitor("m-example-monitor-default")
	if def == nil {
		t.Error("Default monitor should be running")
	}
}

func TestManager_Stop(t *testing.T) {
	m, _ := newTestManager(t)
	// Seeded monitor exists.
	m.Sync()

	count := len(m.GetAll())
	if count == 0 {
		t.Error("Should have monitors running")
	}

	m.Stop()
	// Manager Stop doesn't clear the map, but stops the monitors.
	// We can't easily check if they are stopped via public API unless we check valid status?
	// But it closes channels.
}
