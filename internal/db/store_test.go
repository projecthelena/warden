package db

import (
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test store: %v", err)
	}
	// Clear seeded data
	_, _ = store.db.Exec("DELETE FROM monitor_checks")
	_, _ = store.db.Exec("DELETE FROM monitor_events")
	_, _ = store.db.Exec("DELETE FROM monitors")
	_, _ = store.db.Exec("DELETE FROM groups")
	// Migrate manually if NewStore doesn't do it automatically or if we need fresh state
	// NewStore calls migrate() in implementation? Let's assume it does for now based on previous reads.
	// If NewStore doesn't call migrate, we might fail.
	// Looking at store.go previously: func NewStore(path string) (*Store, error) calls s.migrate().
	return store
}

func TestCreateAndGetMonitor(t *testing.T) {
	s := newTestStore(t)

	// Create Group first
	g := Group{ID: "g1", Name: "Test Group", CreatedAt: time.Now()}
	if err := s.CreateGroup(g); err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}

	m := Monitor{
		ID:        "m1",
		GroupID:   "g1",
		Name:      "Test Monitor",
		URL:       "http://example.com",
		Active:    true,
		Interval:  120, // 2 minutes
		CreatedAt: time.Now(),
	}

	if err := s.CreateMonitor(m); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}

	monitors, err := s.GetMonitors()
	if err != nil {
		t.Fatalf("GetMonitors failed: %v", err)
	}

	if len(monitors) != 1 {
		t.Errorf("Expected 1 monitor, got %d", len(monitors))
	}
	if monitors[0].Interval != 120 {
		t.Errorf("Expected interval 120, got %d", monitors[0].Interval)
	}
}

func TestUpdateMonitor(t *testing.T) {
	s := newTestStore(t)
	s.CreateGroup(Group{ID: "g1", Name: "G1"})
	m := Monitor{ID: "m1", GroupID: "g1", Name: "Old", URL: "http://old.com", Interval: 60}
	s.CreateMonitor(m)

	err := s.UpdateMonitor("m1", "New Name", "http://new.com", 300)
	if err != nil {
		t.Fatalf("UpdateMonitor failed: %v", err)
	}

	monitors, _ := s.GetMonitors()
	if monitors[0].Name != "New Name" {
		t.Errorf("Name not updated")
	}
	if monitors[0].URL != "http://new.com" {
		t.Errorf("URL not updated")
	}
	if monitors[0].Interval != 300 {
		t.Errorf("Interval not updated, got %d", monitors[0].Interval)
	}
}

func TestUpdateMonitor_NotFound(t *testing.T) {
	s := newTestStore(t)
	err := s.UpdateMonitor("missing", "Name", "URL", 60)
	if err == nil {
		t.Error("Expected error for missing monitor, got nil")
	}
	if err.Error() != "monitor not found" {
		t.Errorf("Expected 'monitor not found', got '%v'", err)
	}
}

func TestGetGroupsWithMonitors(t *testing.T) {
	s := newTestStore(t)
	s.CreateGroup(Group{ID: "g1", Name: "G1"})
	s.CreateMonitor(Monitor{ID: "m1", GroupID: "g1", Name: "M1", Interval: 10})
	s.CreateMonitor(Monitor{ID: "m2", GroupID: "g1", Name: "M2", Interval: 20})

	groups, err := s.GetGroups()
	if err != nil {
		t.Fatalf("GetGroups failed: %v", err)
	}

	if len(groups) != 1 {
		t.Fatalf("Expected 1 group, got %d", len(groups))
	}
	if len(groups[0].Monitors) != 2 {
		t.Errorf("Expected 2 monitors in group, got %d", len(groups[0].Monitors))
	}
}

func TestSettingsResult(t *testing.T) {
	s := newTestStore(t)

	val, err := s.GetSetting("missing")
	if err == nil {
		t.Error("Expected error for missing setting")
	}

	if err := s.SetSetting("foo", "bar"); err != nil {
		t.Fatalf("SetSetting failed: %v", err)
	}

	val, err = s.GetSetting("foo")
	if err != nil {
		t.Fatalf("GetSetting failed: %v", err)
	}
	if val != "bar" {
		t.Errorf("Expected 'bar', got '%s'", val)
	}

	// Test Update
	s.SetSetting("foo", "baz")
	val, _ = s.GetSetting("foo")
	if val != "baz" {
		t.Errorf("Expected 'baz', got '%s'", val)
	}
}
