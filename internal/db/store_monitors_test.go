package db

import (
	"testing"
	"time"
)

func TestMonitorCRUD(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreateGroup(Group{ID: "g1", Name: "G1"})

	m := Monitor{
		ID:        "m1",
		GroupID:   "g1",
		Name:      "Monitor 1",
		URL:       "http://test.com",
		Active:    true,
		Interval:  60,
		CreatedAt: time.Now(),
	}

	// Create
	if err := s.CreateMonitor(m); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}

	// Read
	mons, err := s.GetMonitors()
	if err != nil {
		t.Fatalf("GetMonitors failed: %v", err)
	}
	if len(mons) != 1 {
		t.Errorf("Expected 1 monitor (ignoring seeds if cleared), got %d", len(mons))
	}

	// Update
	if err := s.UpdateMonitor("m1", "Updated M1", "http://new.com", 120); err != nil {
		t.Fatalf("UpdateMonitor failed: %v", err)
	}

	// Verify Update
	mons, _ = s.GetMonitors()
	var found *Monitor
	for i := range mons {
		if mons[i].ID == "m1" {
			found = &mons[i]
			break
		}
	}
	if found == nil {
		t.Fatal("Monitor m1 not found")
	}
	if found.Name != "Updated M1" || found.Interval != 120 {
		t.Error("Update verification failed")
	}

	// Delete
	if err := s.DeleteMonitor("m1"); err != nil {
		t.Fatalf("DeleteMonitor failed: %v", err)
	}

	// Verify Delete
	mons, _ = s.GetMonitors()
	// Check m1 is gone
	for _, mn := range mons {
		if mn.ID == "m1" {
			t.Error("Monitor m1 should be deleted")
		}
	}
}

func TestMonitorChecksAndEvents(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreateGroup(Group{ID: "g1", Name: "G1"})
	_ = s.CreateMonitor(Monitor{ID: "m1", GroupID: "g1", Name: "M1", Interval: 60})

	// Checks
	checks := []CheckResult{
		{MonitorID: "m1", Status: "up", Latency: 50, Timestamp: time.Now(), StatusCode: 200},
	}
	if err := s.BatchInsertChecks(checks); err != nil {
		t.Fatalf("BatchInsertChecks failed: %v", err)
	}

	history, err := s.GetMonitorChecks("m1", 10)
	if err != nil {
		t.Fatalf("GetMonitorChecks failed: %v", err)
	}
	if len(history) != 1 {
		t.Errorf("Expected 1 check, got %d", len(history))
	}

	// Events
	if err := s.CreateEvent("m1", "up", "It is up"); err != nil {
		t.Fatalf("CreateEvent failed: %v", err)
	}
	events, err := s.GetMonitorEvents("m1", 10)
	if err != nil {
		t.Fatalf("GetMonitorEvents failed: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(events))
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

	// 4. Test Monitor Deletion Cascade
	if err := s.DeleteMonitor("m-del"); err != nil {
		t.Fatalf("DeleteMonitor failed: %v", err)
	}

	// Verify Data Gone
	cHistory, _ := s.GetMonitorChecks("m-del", 10)
	if len(cHistory) != 0 {
		t.Errorf("Expected 0 checks after monitor deletion, got %d", len(cHistory))
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

	// 3. Close Outage
	if err := s.CloseOutage("m1"); err != nil {
		t.Fatalf("CloseOutage failed: %v", err)
	}

	// 4. Verify No Active
	active, _ = s.GetActiveOutages()
	if len(active) != 0 {
		t.Errorf("Expected 0 active outages, got %d", len(active))
	}

	// 5. Verify Resolved History
	// Needs careful time handling in test environment vs sqlite precision usually.
	// But standard logic implies it worked if active is 0.
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

func TestGetActiveSSLWarnings_Empty(t *testing.T) {
	s := newTestStore(t)

	// No monitors, no events
	warnings, err := s.GetActiveSSLWarnings()
	if err != nil {
		t.Fatalf("GetActiveSSLWarnings failed: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("Expected 0 warnings, got %d", len(warnings))
	}
}

func TestGetActiveSSLWarnings_SingleMonitor(t *testing.T) {
	s := newTestStore(t)

	// Setup
	if err := s.CreateGroup(Group{ID: "g1", Name: "Production"}); err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}
	if err := s.CreateMonitor(Monitor{ID: "m1", GroupID: "g1", Name: "API Server", URL: "https://api.example.com", Interval: 60}); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}

	// Create SSL expiring event
	if err := s.CreateEvent("m1", "ssl_expiring", "SSL certificate expires in 14 days (2025-02-15)"); err != nil {
		t.Fatalf("CreateEvent failed: %v", err)
	}

	// Verify
	warnings, err := s.GetActiveSSLWarnings()
	if err != nil {
		t.Fatalf("GetActiveSSLWarnings failed: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("Expected 1 warning, got %d", len(warnings))
	}
	if warnings[0].MonitorID != "m1" {
		t.Errorf("Expected MonitorID m1, got %s", warnings[0].MonitorID)
	}
	if warnings[0].MonitorName != "API Server" {
		t.Errorf("Expected MonitorName 'API Server', got %s", warnings[0].MonitorName)
	}
	if warnings[0].GroupName != "Production" {
		t.Errorf("Expected GroupName 'Production', got %s", warnings[0].GroupName)
	}
	if warnings[0].GroupID != "g1" {
		t.Errorf("Expected GroupID g1, got %s", warnings[0].GroupID)
	}
	if warnings[0].Message != "SSL certificate expires in 14 days (2025-02-15)" {
		t.Errorf("Unexpected message: %s", warnings[0].Message)
	}
}

func TestGetActiveSSLWarnings_MultipleMonitors(t *testing.T) {
	s := newTestStore(t)

	// Setup
	if err := s.CreateGroup(Group{ID: "g1", Name: "Production"}); err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}
	if err := s.CreateGroup(Group{ID: "g2", Name: "Staging"}); err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}
	if err := s.CreateMonitor(Monitor{ID: "m1", GroupID: "g1", Name: "API", Interval: 60}); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}
	if err := s.CreateMonitor(Monitor{ID: "m2", GroupID: "g1", Name: "Web", Interval: 60}); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}
	if err := s.CreateMonitor(Monitor{ID: "m3", GroupID: "g2", Name: "Staging API", Interval: 60}); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}

	// Create SSL events for different monitors
	_ = s.CreateEvent("m1", "ssl_expiring", "SSL certificate expires in 7 days")
	_ = s.CreateEvent("m2", "ssl_expiring", "SSL certificate expires in 30 days")
	_ = s.CreateEvent("m3", "ssl_expiring", "SSL certificate expires in 1 day")

	warnings, err := s.GetActiveSSLWarnings()
	if err != nil {
		t.Fatalf("GetActiveSSLWarnings failed: %v", err)
	}
	if len(warnings) != 3 {
		t.Fatalf("Expected 3 warnings, got %d", len(warnings))
	}

	// Verify all monitors are represented
	monitorIDs := make(map[string]bool)
	for _, w := range warnings {
		monitorIDs[w.MonitorID] = true
	}
	if !monitorIDs["m1"] || !monitorIDs["m2"] || !monitorIDs["m3"] {
		t.Error("Expected all 3 monitors in warnings")
	}
}

func TestGetActiveSSLWarnings_OnlyLatestPerMonitor(t *testing.T) {
	s := newTestStore(t)

	// Setup
	if err := s.CreateGroup(Group{ID: "g1", Name: "G1"}); err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}
	if err := s.CreateMonitor(Monitor{ID: "m1", GroupID: "g1", Name: "M1", Interval: 60}); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}

	// Create multiple SSL events for same monitor (simulating different threshold notifications)
	_ = s.CreateEvent("m1", "ssl_expiring", "SSL certificate expires in 30 days")
	_ = s.CreateEvent("m1", "ssl_expiring", "SSL certificate expires in 14 days")
	_ = s.CreateEvent("m1", "ssl_expiring", "SSL certificate expires in 7 days")

	warnings, err := s.GetActiveSSLWarnings()
	if err != nil {
		t.Fatalf("GetActiveSSLWarnings failed: %v", err)
	}

	// Should only return the latest event per monitor
	if len(warnings) != 1 {
		t.Fatalf("Expected 1 warning (latest only), got %d", len(warnings))
	}
	if warnings[0].Message != "SSL certificate expires in 7 days" {
		t.Errorf("Expected latest message '...7 days', got %s", warnings[0].Message)
	}
}

func TestGetActiveSSLWarnings_IgnoresOtherEventTypes(t *testing.T) {
	s := newTestStore(t)

	// Setup
	if err := s.CreateGroup(Group{ID: "g1", Name: "G1"}); err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}
	if err := s.CreateMonitor(Monitor{ID: "m1", GroupID: "g1", Name: "M1", Interval: 60}); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}

	// Create different event types
	_ = s.CreateEvent("m1", "down", "Monitor is down")
	_ = s.CreateEvent("m1", "degraded", "High latency")
	_ = s.CreateEvent("m1", "recovered", "Monitor recovered")
	_ = s.CreateEvent("m1", "ssl_expiring", "SSL certificate expires in 7 days")

	warnings, err := s.GetActiveSSLWarnings()
	if err != nil {
		t.Fatalf("GetActiveSSLWarnings failed: %v", err)
	}

	// Should only return ssl_expiring events
	if len(warnings) != 1 {
		t.Fatalf("Expected 1 warning, got %d", len(warnings))
	}
	if warnings[0].Message != "SSL certificate expires in 7 days" {
		t.Errorf("Unexpected message: %s", warnings[0].Message)
	}
}

func TestGetActiveSSLWarnings_DeletedMonitorExcluded(t *testing.T) {
	s := newTestStore(t)

	// Setup
	if err := s.CreateGroup(Group{ID: "g1", Name: "G1"}); err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}
	if err := s.CreateMonitor(Monitor{ID: "m1", GroupID: "g1", Name: "M1", Interval: 60}); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}
	if err := s.CreateMonitor(Monitor{ID: "m2", GroupID: "g1", Name: "M2", Interval: 60}); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}

	// Create SSL events
	_ = s.CreateEvent("m1", "ssl_expiring", "SSL expiring m1")
	_ = s.CreateEvent("m2", "ssl_expiring", "SSL expiring m2")

	// Delete one monitor (cascades to events)
	if err := s.DeleteMonitor("m1"); err != nil {
		t.Fatalf("DeleteMonitor failed: %v", err)
	}

	warnings, err := s.GetActiveSSLWarnings()
	if err != nil {
		t.Fatalf("GetActiveSSLWarnings failed: %v", err)
	}

	// Should only return m2's warning
	if len(warnings) != 1 {
		t.Fatalf("Expected 1 warning, got %d", len(warnings))
	}
	if warnings[0].MonitorID != "m2" {
		t.Errorf("Expected MonitorID m2, got %s", warnings[0].MonitorID)
	}
}

func TestSetMonitorActive_PauseMonitor(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreateGroup(Group{ID: "g1", Name: "G1"})

	// Create an active monitor
	m := Monitor{
		ID:       "m1",
		GroupID:  "g1",
		Name:     "Test Monitor",
		URL:      "http://example.com",
		Active:   true,
		Interval: 60,
	}
	if err := s.CreateMonitor(m); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}

	// Verify it's initially active
	monitors, _ := s.GetMonitors()
	var found *Monitor
	for i := range monitors {
		if monitors[i].ID == "m1" {
			found = &monitors[i]
			break
		}
	}
	if found == nil {
		t.Fatal("Monitor not found")
	}
	if !found.Active {
		t.Error("Monitor should be active initially")
	}

	// Pause the monitor
	if err := s.SetMonitorActive("m1", false); err != nil {
		t.Fatalf("SetMonitorActive(false) failed: %v", err)
	}

	// Verify it's now inactive
	monitors, _ = s.GetMonitors()
	for i := range monitors {
		if monitors[i].ID == "m1" {
			found = &monitors[i]
			break
		}
	}
	if found.Active {
		t.Error("Monitor should be inactive after pause")
	}
}

func TestSetMonitorActive_ResumeMonitor(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreateGroup(Group{ID: "g1", Name: "G1"})

	// Create an inactive monitor
	m := Monitor{
		ID:       "m1",
		GroupID:  "g1",
		Name:     "Test Monitor",
		URL:      "http://example.com",
		Active:   false, // Start as inactive
		Interval: 60,
	}
	if err := s.CreateMonitor(m); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}

	// Resume the monitor
	if err := s.SetMonitorActive("m1", true); err != nil {
		t.Fatalf("SetMonitorActive(true) failed: %v", err)
	}

	// Verify it's now active
	monitors, _ := s.GetMonitors()
	var found *Monitor
	for i := range monitors {
		if monitors[i].ID == "m1" {
			found = &monitors[i]
			break
		}
	}
	if found == nil {
		t.Fatal("Monitor not found")
	}
	if !found.Active {
		t.Error("Monitor should be active after resume")
	}
}

func TestSetMonitorActive_NotFound(t *testing.T) {
	s := newTestStore(t)

	// Try to pause a non-existent monitor
	err := s.SetMonitorActive("non-existent", false)
	if err == nil {
		t.Fatal("Expected error for non-existent monitor")
	}
	if err != ErrMonitorNotFound {
		t.Errorf("Expected ErrMonitorNotFound, got: %v", err)
	}
}

func TestSetMonitorActive_Idempotent(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreateGroup(Group{ID: "g1", Name: "G1"})

	m := Monitor{
		ID:       "m1",
		GroupID:  "g1",
		Name:     "Test Monitor",
		URL:      "http://example.com",
		Active:   true,
		Interval: 60,
	}
	if err := s.CreateMonitor(m); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}

	// Pause twice - should be idempotent
	if err := s.SetMonitorActive("m1", false); err != nil {
		t.Fatalf("First pause failed: %v", err)
	}
	if err := s.SetMonitorActive("m1", false); err != nil {
		t.Fatalf("Second pause failed: %v", err)
	}

	// Resume twice - should be idempotent
	if err := s.SetMonitorActive("m1", true); err != nil {
		t.Fatalf("First resume failed: %v", err)
	}
	if err := s.SetMonitorActive("m1", true); err != nil {
		t.Fatalf("Second resume failed: %v", err)
	}

	// Verify final state
	monitors, _ := s.GetMonitors()
	var found *Monitor
	for i := range monitors {
		if monitors[i].ID == "m1" {
			found = &monitors[i]
			break
		}
	}
	if !found.Active {
		t.Error("Monitor should be active after final resume")
	}
}

func TestSetMonitorActive_EmptyID(t *testing.T) {
	s := newTestStore(t)

	// Empty ID should return ErrMonitorNotFound
	err := s.SetMonitorActive("", false)
	if err == nil {
		t.Fatal("Expected error for empty ID")
	}
	if err != ErrMonitorNotFound {
		t.Errorf("Expected ErrMonitorNotFound, got: %v", err)
	}
}

func TestSetMonitorActive_SpecialCharactersInID(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreateGroup(Group{ID: "g1", Name: "G1"})

	// UUID-like IDs are commonly used - verify they work
	specialID := "mon-abc123-def456-789xyz"
	m := Monitor{
		ID:       specialID,
		GroupID:  "g1",
		Name:     "Special ID Monitor",
		URL:      "http://example.com",
		Active:   true,
		Interval: 60,
	}
	if err := s.CreateMonitor(m); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}

	// Pause with special ID
	if err := s.SetMonitorActive(specialID, false); err != nil {
		t.Fatalf("Pause failed: %v", err)
	}

	// Verify
	monitors, _ := s.GetMonitors()
	for _, mon := range monitors {
		if mon.ID == specialID && mon.Active {
			t.Error("Monitor should be inactive")
		}
	}
}

func TestSetMonitorActive_RapidToggle(t *testing.T) {
	// Test rapid sequential pause/resume operations
	s := newTestStore(t)
	if err := s.CreateGroup(Group{ID: "g1", Name: "G1"}); err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}

	// Create monitor
	m := Monitor{
		ID:       "m-toggle",
		GroupID:  "g1",
		Name:     "Toggle Test",
		URL:      "http://example.com",
		Active:   true,
		Interval: 60,
	}
	if err := s.CreateMonitor(m); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}

	// Run rapid sequential pause/resume operations
	for i := 0; i < 20; i++ {
		if err := s.SetMonitorActive("m-toggle", false); err != nil {
			t.Fatalf("Pause %d failed: %v", i, err)
		}
		if err := s.SetMonitorActive("m-toggle", true); err != nil {
			t.Fatalf("Resume %d failed: %v", i, err)
		}
	}

	// Monitor should still exist and be active
	monitors, err := s.GetMonitors()
	if err != nil {
		t.Fatalf("GetMonitors failed: %v", err)
	}
	found := false
	for _, mon := range monitors {
		if mon.ID == "m-toggle" {
			found = true
			if !mon.Active {
				t.Error("Monitor should be active after final toggle")
			}
			break
		}
	}
	if !found {
		t.Errorf("Monitor should still exist after toggle operations, got %d monitors total", len(monitors))
	}
}

func TestSetMonitorActive_DeletedMonitor(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreateGroup(Group{ID: "g1", Name: "G1"})

	m := Monitor{
		ID:       "m-delete",
		GroupID:  "g1",
		Name:     "Delete Test",
		URL:      "http://example.com",
		Active:   true,
		Interval: 60,
	}
	if err := s.CreateMonitor(m); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}

	// Delete the monitor
	if err := s.DeleteMonitor("m-delete"); err != nil {
		t.Fatalf("DeleteMonitor failed: %v", err)
	}

	// Try to pause deleted monitor - should return not found
	err := s.SetMonitorActive("m-delete", false)
	if err == nil {
		t.Fatal("Expected error for deleted monitor")
	}
	if err != ErrMonitorNotFound {
		t.Errorf("Expected ErrMonitorNotFound, got: %v", err)
	}
}

func TestSetMonitorActive_PausePreservesOtherFields(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreateGroup(Group{ID: "g1", Name: "G1"})

	m := Monitor{
		ID:       "m-preserve",
		GroupID:  "g1",
		Name:     "Preserve Test",
		URL:      "http://example.com",
		Active:   true,
		Interval: 120,
	}
	if err := s.CreateMonitor(m); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}

	// Pause
	if err := s.SetMonitorActive("m-preserve", false); err != nil {
		t.Fatalf("Pause failed: %v", err)
	}

	// Verify other fields are preserved
	monitors, _ := s.GetMonitors()
	var found *Monitor
	for i := range monitors {
		if monitors[i].ID == "m-preserve" {
			found = &monitors[i]
			break
		}
	}
	if found == nil {
		t.Fatal("Monitor not found")
	}
	if found.Name != "Preserve Test" {
		t.Errorf("Name changed unexpectedly: %s", found.Name)
	}
	if found.URL != "http://example.com" {
		t.Errorf("URL changed unexpectedly: %s", found.URL)
	}
	if found.Interval != 120 {
		t.Errorf("Interval changed unexpectedly: %d", found.Interval)
	}
	if found.GroupID != "g1" {
		t.Errorf("GroupID changed unexpectedly: %s", found.GroupID)
	}
}
