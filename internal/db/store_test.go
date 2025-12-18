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
	if err := s.CreateGroup(Group{ID: "g1", Name: "G1"}); err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}
	m := Monitor{ID: "m1", GroupID: "g1", Name: "Old", URL: "http://old.com", Interval: 60}
	if err := s.CreateMonitor(m); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}

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
	if err := s.CreateGroup(Group{ID: "g1", Name: "G1"}); err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}
	if err := s.CreateMonitor(Monitor{ID: "m1", GroupID: "g1", Name: "M1", Interval: 10}); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}
	if err := s.CreateMonitor(Monitor{ID: "m2", GroupID: "g1", Name: "M2", Interval: 20}); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}

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

	_, err := s.GetSetting("missing")
	if err == nil {
		t.Error("Expected error for missing setting")
	}

	if err := s.SetSetting("foo", "bar"); err != nil {
		t.Fatalf("SetSetting failed: %v", err)
	}

	val, err := s.GetSetting("foo")
	if err != nil {
		t.Fatalf("GetSetting failed: %v", err)
	}
	if val != "bar" {
		t.Errorf("Expected 'bar', got '%s'", val)
	}

	_ = s.SetSetting("foo", "baz")
	val, _ = s.GetSetting("foo")
	if val != "baz" {
		t.Errorf("Expected 'baz', got '%s'", val)
	}
}

func TestMonitorOutages(t *testing.T) {
	s := newTestStore(t)
	// Create Group & Monitor Dependencies
	if err := s.CreateGroup(Group{ID: "g1", Name: "G1"}); err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}
	if err := s.CreateMonitor(Monitor{ID: "m1", GroupID: "g1", Name: "M1", Interval: 60}); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}

	// 1. Create Outage
	if err := s.CreateOutage("m1", "down", "Connection refused"); err != nil {
		t.Fatalf("CreateOutage failed: %v", err)
	}

	// 2. Verify Active
	active, err := s.GetActiveOutages()
	if err != nil {
		t.Fatalf("GetActiveOutages failed: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("Expected 1 active outage, got %d", len(active))
	}
	if active[0].MonitorID != "m1" {
		t.Errorf("Expected monitor ID m1, got %s", active[0].MonitorID)
	}
	if active[0].Type != "down" {
		t.Errorf("Expected type down, got %s", active[0].Type)
	}
	if active[0].GroupName != "G1" {
		t.Errorf("Expected group name G1, got %s", active[0].GroupName)
	}

	// 3. Close Outage
	time.Sleep(1 * time.Millisecond) // Ensure time difference
	if err := s.CloseOutage("m1"); err != nil {
		t.Fatalf("CloseOutage failed: %v", err)
	}

	// 4. Verify No Active
	active, _ = s.GetActiveOutages()
	if len(active) != 0 {
		t.Errorf("Expected 0 active outages, got %d", len(active))
	}

	// 5. Verify Resolved History
	history, err := s.GetResolvedOutages(time.Time{})
	if err != nil {
		t.Fatalf("GetResolvedOutages failed: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("Expected 1 history item, got %d", len(history))
	}
	if history[0].EndTime == nil {
		t.Error("Expected EndTime to be set")
	}
}

func TestNotificationChannels(t *testing.T) {
	s := newTestStore(t)

	c := NotificationChannel{
		ID:        "nc1",
		Type:      "slack",
		Name:      "Dev Team",
		Config:    `{"webhookUrl": "https://hooks.slack.com/..."}`,
		Enabled:   true,
		CreatedAt: time.Now(),
	}

	// Create
	if err := s.CreateNotificationChannel(c); err != nil {
		t.Fatalf("CreateNotificationChannel failed: %v", err)
	}

	// Get
	channels, err := s.GetNotificationChannels()
	if err != nil {
		t.Fatalf("GetNotificationChannels failed: %v", err)
	}
	if len(channels) != 1 {
		t.Fatalf("Expected 1 channel, got %d", len(channels))
	}
	if channels[0].Name != "Dev Team" {
		t.Errorf("Expected name 'Dev Team', got '%s'", channels[0].Name)
	}

	// Delete
	if err := s.DeleteNotificationChannel("nc1"); err != nil {
		t.Fatalf("DeleteNotificationChannel failed: %v", err)
	}

	// Verify Empty
	channels, _ = s.GetNotificationChannels()
	if len(channels) != 0 {
		t.Errorf("Expected 0 channels, got %d", len(channels))
	}
}

func TestCascadingDeletion(t *testing.T) {
	s := newTestStore(t)

	// 1. Create Group
	err := s.CreateGroup(Group{ID: "g-del", Name: "To Delete"})
	if err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}

	// 2. Create Monitor
	m := Monitor{
		ID:       "m-del",
		GroupID:  "g-del",
		Name:     "Monitor To Delete",
		URL:      "http://example.com",
		Interval: 60,
	}
	if err := s.CreateMonitor(m); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}

	// 3. Add Data (Checks & Events)
	checks := []CheckResult{
		{MonitorID: "m-del", Status: "up", Latency: 100, Timestamp: time.Now()},
	}
	if err := s.BatchInsertChecks(checks); err != nil {
		t.Fatalf("BatchInsertChecks failed: %v", err)
	}

	if err := s.CreateEvent("m-del", "down", "It died"); err != nil {
		t.Fatalf("CreateEvent failed: %v", err)
	}

	// Verify data exists
	cHistory, _ := s.GetMonitorChecks("m-del", 10)
	if len(cHistory) == 0 {
		t.Fatal("Setup failed: no checks found")
	}

	// 4. Test Monitor Deletion Cascade
	if err := s.DeleteMonitor("m-del"); err != nil {
		t.Fatalf("DeleteMonitor failed: %v", err)
	}

	// Verify Data Gone
	cHistory, _ = s.GetMonitorChecks("m-del", 10)
	if len(cHistory) != 0 {
		t.Errorf("Expected 0 checks after monitor deletion, got %d", len(cHistory))
	}

	// Verify Event Gone? (Need a way to check events directly or assume FK works if checks worked)
	var eventCount int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM monitor_events WHERE monitor_id = 'm-del'").Scan(&eventCount); err != nil {
		t.Fatalf("Failed to scan event count: %v", err)
	}
	if eventCount != 0 {
		t.Errorf("Expected 0 events after monitor deletion, got %d", eventCount)
	}

	// 5. Test Group Deletion Cascade
	// Re-create monitor and data

	// Need to handle unique constraints if ID reused? SQLite usually OK if deleted.
	// Re-create Group? No, Group g-del still exists.

	err = s.CreateMonitor(m)
	if err != nil {
		t.Fatalf("Re-create monitor failed: %v", err)
	}
	if err := s.BatchInsertChecks(checks); err != nil {
		t.Fatalf("BatchInsertChecks failed: %v", err)
	}

	// Delete Group
	if err := s.DeleteGroup("g-del"); err != nil {
		t.Fatalf("DeleteGroup failed: %v", err)
	}

	// Verify Monitor Gone
	monitors, _ := s.GetMonitors()
	for _, mon := range monitors {
		if mon.ID == "m-del" {
			t.Error("Monitor m-del should have been deleted via cascade")
		}
	}

	// Verify Data Gone Again
	cHistory, _ = s.GetMonitorChecks("m-del", 10)
	if len(cHistory) != 0 {
		t.Errorf("Expected 0 checks after group deletion, got %d", len(cHistory))
	}
}
