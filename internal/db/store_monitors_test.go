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
	if err := s.UpdateMonitor("m1", "Updated M1", "http://new.com", 120, nil, nil); err != nil {
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

// ============== DAILY UPTIME STATS TESTS ==============

func TestGetDailyUptimeStats_Empty(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreateGroup(Group{ID: "g1", Name: "G1"})
	_ = s.CreateMonitor(Monitor{ID: "m1", GroupID: "g1", Name: "M1", Interval: 60})

	stats, err := s.GetDailyUptimeStats("m1", 7)
	if err != nil {
		t.Fatalf("GetDailyUptimeStats failed: %v", err)
	}
	if len(stats) != 7 {
		t.Fatalf("Expected 7 days, got %d", len(stats))
	}
	// All days should have no data
	for _, d := range stats {
		if d.Total != 0 {
			t.Errorf("Expected 0 total checks for empty monitor, got %d on %s", d.Total, d.Date)
		}
		if d.UptimePercent != -1 {
			t.Errorf("Expected -1 uptime for no-data day, got %f on %s", d.UptimePercent, d.Date)
		}
	}
}

func TestGetDailyUptimeStats_WithChecks(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreateGroup(Group{ID: "g1", Name: "G1"})
	_ = s.CreateMonitor(Monitor{ID: "m1", GroupID: "g1", Name: "M1", Interval: 60})

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, time.UTC)

	// Insert checks: 8 up + 2 down = 80% uptime today
	var checks []CheckResult
	for i := 0; i < 8; i++ {
		checks = append(checks, CheckResult{
			MonitorID: "m1", Status: "up", Latency: 50,
			Timestamp: today.Add(time.Duration(i) * time.Minute), StatusCode: 200,
		})
	}
	for i := 0; i < 2; i++ {
		checks = append(checks, CheckResult{
			MonitorID: "m1", Status: "down", Latency: 0,
			Timestamp: today.Add(time.Duration(8+i) * time.Minute), StatusCode: 0,
		})
	}
	if err := s.BatchInsertChecks(checks); err != nil {
		t.Fatalf("BatchInsertChecks failed: %v", err)
	}

	stats, err := s.GetDailyUptimeStats("m1", 7)
	if err != nil {
		t.Fatalf("GetDailyUptimeStats failed: %v", err)
	}
	if len(stats) != 7 {
		t.Fatalf("Expected 7 days, got %d", len(stats))
	}

	// The last day (today) should have data
	todayStr := today.Format("2006-01-02")
	var todayStat *DailyUptimeStat
	for i := range stats {
		if stats[i].Date == todayStr {
			todayStat = &stats[i]
			break
		}
	}
	if todayStat == nil {
		t.Fatalf("Today's stats not found in results. Looking for %s", todayStr)
	}
	if todayStat.Total != 10 {
		t.Errorf("Expected 10 total checks today, got %d", todayStat.Total)
	}
	if todayStat.UptimePercent != 80.0 {
		t.Errorf("Expected 80%% uptime today, got %.2f%%", todayStat.UptimePercent)
	}
}

func TestGetDailyUptimeStats_MultipleMonitors(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreateGroup(Group{ID: "g1", Name: "G1"})
	_ = s.CreateMonitor(Monitor{ID: "m1", GroupID: "g1", Name: "M1", Interval: 60})
	_ = s.CreateMonitor(Monitor{ID: "m2", GroupID: "g1", Name: "M2", Interval: 60})

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, time.UTC)

	checks := []CheckResult{
		{MonitorID: "m1", Status: "up", Latency: 50, Timestamp: today, StatusCode: 200},
		{MonitorID: "m2", Status: "down", Latency: 0, Timestamp: today, StatusCode: 0},
	}
	if err := s.BatchInsertChecks(checks); err != nil {
		t.Fatalf("BatchInsertChecks failed: %v", err)
	}

	// m1 should be 100%
	stats1, _ := s.GetDailyUptimeStats("m1", 1)
	if len(stats1) != 1 || stats1[0].UptimePercent != 100.0 {
		t.Errorf("m1 expected 100%%, got %.2f%%", stats1[0].UptimePercent)
	}

	// m2 should be 0%
	stats2, _ := s.GetDailyUptimeStats("m2", 1)
	if len(stats2) != 1 || stats2[0].UptimePercent != 0.0 {
		t.Errorf("m2 expected 0%%, got %.2f%%", stats2[0].UptimePercent)
	}
}

func TestGetDailyUptimeStats_InvalidDays(t *testing.T) {
	s := newTestStore(t)

	_, err := s.GetDailyUptimeStats("m1", 0)
	if err == nil {
		t.Error("Expected error for 0 days")
	}

	_, err = s.GetDailyUptimeStats("m1", 366)
	if err == nil {
		t.Error("Expected error for 366 days")
	}
}

func TestGetDailyUptimeStats_GapFilling(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreateGroup(Group{ID: "g1", Name: "G1"})
	_ = s.CreateMonitor(Monitor{ID: "m1", GroupID: "g1", Name: "M1", Interval: 60})

	// Insert checks only for today (leaving gaps in the 7-day window)
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, time.UTC)
	checks := []CheckResult{
		{MonitorID: "m1", Status: "up", Latency: 50, Timestamp: today, StatusCode: 200},
	}
	if err := s.BatchInsertChecks(checks); err != nil {
		t.Fatalf("BatchInsertChecks failed: %v", err)
	}

	stats, err := s.GetDailyUptimeStats("m1", 7)
	if err != nil {
		t.Fatalf("GetDailyUptimeStats failed: %v", err)
	}

	todayStr := today.Format("2006-01-02")
	daysWithData := 0
	daysWithNoData := 0
	for _, d := range stats {
		if d.Date == todayStr {
			daysWithData++
			if d.Total != 1 {
				t.Errorf("Expected 1 check on %s, got %d", d.Date, d.Total)
			}
		} else {
			daysWithNoData++
			if d.UptimePercent != -1 {
				t.Errorf("Expected -1 (no data) on %s, got %.2f", d.Date, d.UptimePercent)
			}
		}
	}
	if daysWithData != 1 {
		t.Errorf("Expected exactly 1 day with data, got %d", daysWithData)
	}
	if daysWithNoData != 6 {
		t.Errorf("Expected 6 days with no data, got %d", daysWithNoData)
	}
}

// ============== PER-MONITOR OVERRIDE CRUD TESTS ==============

func intPtr(v int) *int { return &v }

func TestMonitor_PerMonitorOverrides(t *testing.T) {
	t.Run("create_with_overrides", func(t *testing.T) {
		s := newTestStore(t)
		_ = s.CreateGroup(Group{ID: "g1", Name: "G1"})

		m := Monitor{
			ID:                      "m-ov1",
			GroupID:                 "g1",
			Name:                    "Override Create",
			URL:                     "http://example.com",
			Active:                  true,
			Interval:                60,
			ConfirmationThreshold:   intPtr(5),
			NotificationCooldownMin: intPtr(10),
		}
		if err := s.CreateMonitor(m); err != nil {
			t.Fatalf("CreateMonitor failed: %v", err)
		}

		mons, err := s.GetMonitors()
		if err != nil {
			t.Fatalf("GetMonitors failed: %v", err)
		}
		var found *Monitor
		for i := range mons {
			if mons[i].ID == "m-ov1" {
				found = &mons[i]
				break
			}
		}
		if found == nil {
			t.Fatal("Monitor not found")
		}
		if found.ConfirmationThreshold == nil || *found.ConfirmationThreshold != 5 {
			t.Errorf("Expected ConfirmationThreshold=5, got %v", found.ConfirmationThreshold)
		}
		if found.NotificationCooldownMin == nil || *found.NotificationCooldownMin != 10 {
			t.Errorf("Expected NotificationCooldownMin=10, got %v", found.NotificationCooldownMin)
		}
	})

	t.Run("create_without_overrides", func(t *testing.T) {
		s := newTestStore(t)
		_ = s.CreateGroup(Group{ID: "g1", Name: "G1"})

		m := Monitor{
			ID:       "m-ov2",
			GroupID:  "g1",
			Name:     "No Override",
			URL:      "http://example.com",
			Active:   true,
			Interval: 60,
		}
		if err := s.CreateMonitor(m); err != nil {
			t.Fatalf("CreateMonitor failed: %v", err)
		}

		mons, _ := s.GetMonitors()
		var found *Monitor
		for i := range mons {
			if mons[i].ID == "m-ov2" {
				found = &mons[i]
				break
			}
		}
		if found == nil {
			t.Fatal("Monitor not found")
		}
		if found.ConfirmationThreshold != nil {
			t.Errorf("Expected ConfirmationThreshold=nil, got %v", *found.ConfirmationThreshold)
		}
		if found.NotificationCooldownMin != nil {
			t.Errorf("Expected NotificationCooldownMin=nil, got %v", *found.NotificationCooldownMin)
		}
	})

	t.Run("update_adds_overrides", func(t *testing.T) {
		s := newTestStore(t)
		_ = s.CreateGroup(Group{ID: "g1", Name: "G1"})

		// Create with no overrides
		m := Monitor{
			ID: "m-ov3", GroupID: "g1", Name: "Add Override",
			URL: "http://example.com", Active: true, Interval: 60,
		}
		if err := s.CreateMonitor(m); err != nil {
			t.Fatalf("CreateMonitor failed: %v", err)
		}

		// Update to add overrides
		if err := s.UpdateMonitor("m-ov3", "Add Override", "http://example.com", 60, intPtr(7), intPtr(15)); err != nil {
			t.Fatalf("UpdateMonitor failed: %v", err)
		}

		mons, _ := s.GetMonitors()
		var found *Monitor
		for i := range mons {
			if mons[i].ID == "m-ov3" {
				found = &mons[i]
				break
			}
		}
		if found == nil {
			t.Fatal("Monitor not found")
		}
		if found.ConfirmationThreshold == nil || *found.ConfirmationThreshold != 7 {
			t.Errorf("Expected ConfirmationThreshold=7, got %v", found.ConfirmationThreshold)
		}
		if found.NotificationCooldownMin == nil || *found.NotificationCooldownMin != 15 {
			t.Errorf("Expected NotificationCooldownMin=15, got %v", found.NotificationCooldownMin)
		}
	})

	t.Run("update_clears_overrides", func(t *testing.T) {
		s := newTestStore(t)
		_ = s.CreateGroup(Group{ID: "g1", Name: "G1"})

		// Create with overrides
		m := Monitor{
			ID: "m-ov4", GroupID: "g1", Name: "Clear Override",
			URL: "http://example.com", Active: true, Interval: 60,
			ConfirmationThreshold:   intPtr(5),
			NotificationCooldownMin: intPtr(10),
		}
		if err := s.CreateMonitor(m); err != nil {
			t.Fatalf("CreateMonitor failed: %v", err)
		}

		// Update to clear overrides
		if err := s.UpdateMonitor("m-ov4", "Clear Override", "http://example.com", 60, nil, nil); err != nil {
			t.Fatalf("UpdateMonitor failed: %v", err)
		}

		mons, _ := s.GetMonitors()
		var found *Monitor
		for i := range mons {
			if mons[i].ID == "m-ov4" {
				found = &mons[i]
				break
			}
		}
		if found == nil {
			t.Fatal("Monitor not found")
		}
		if found.ConfirmationThreshold != nil {
			t.Errorf("Expected ConfirmationThreshold=nil after clear, got %v", *found.ConfirmationThreshold)
		}
		if found.NotificationCooldownMin != nil {
			t.Errorf("Expected NotificationCooldownMin=nil after clear, got %v", *found.NotificationCooldownMin)
		}
	})

	t.Run("partial_override", func(t *testing.T) {
		s := newTestStore(t)
		_ = s.CreateGroup(Group{ID: "g1", Name: "G1"})

		// Create with both overrides
		m := Monitor{
			ID: "m-ov5", GroupID: "g1", Name: "Partial Override",
			URL: "http://example.com", Active: true, Interval: 60,
			ConfirmationThreshold:   intPtr(5),
			NotificationCooldownMin: intPtr(10),
		}
		if err := s.CreateMonitor(m); err != nil {
			t.Fatalf("CreateMonitor failed: %v", err)
		}

		// Update only threshold, clear cooldown
		if err := s.UpdateMonitor("m-ov5", "Partial Override", "http://example.com", 60, intPtr(8), nil); err != nil {
			t.Fatalf("UpdateMonitor failed: %v", err)
		}

		mons, _ := s.GetMonitors()
		var found *Monitor
		for i := range mons {
			if mons[i].ID == "m-ov5" {
				found = &mons[i]
				break
			}
		}
		if found == nil {
			t.Fatal("Monitor not found")
		}
		if found.ConfirmationThreshold == nil || *found.ConfirmationThreshold != 8 {
			t.Errorf("Expected ConfirmationThreshold=8, got %v", found.ConfirmationThreshold)
		}
		if found.NotificationCooldownMin != nil {
			t.Errorf("Expected NotificationCooldownMin=nil, got %v", *found.NotificationCooldownMin)
		}
	})
}
