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
	if err := s.CreateMonitor(mon); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}

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
	if err := s.UpdateMonitor("m-test-1", "Test Monitor", "http://example.com", 120); err != nil {
		t.Fatalf("Failed to update monitor: %v", err)
	}

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

}

func TestManager_Stop(t *testing.T) {
	m, s := newTestManager(t)

	if err := s.CreateMonitor(db.Monitor{
		ID:       "m-stop",
		GroupID:  "g-default",
		Name:     "Stop Test",
		URL:      "http://example.com",
		Active:   true,
		Interval: 60,
	}); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}

	m.Sync()

	count := len(m.GetAll())
	if count == 0 {
		t.Error("Should have monitors running")
	}

	m.Stop()
	// We can't easily check if they are stopped via public API unless we check valid status?
	// But it closes channels.
}

func TestManager_OutageLogic(t *testing.T) {
	m, s := newTestManager(t)
	if err := s.CreateGroup(db.Group{ID: "g-test", Name: "Test Group"}); err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}

	// 1. Create Monitor
	mon := db.Monitor{
		ID:       "m-fail",
		GroupID:  "g-test",
		Name:     "Failing Monitor",
		URL:      "http://127.0.0.1:48201", // Closed port, connection refused
		Active:   true,
		Interval: 1, // Fast interval
	}
	if err := s.CreateMonitor(mon); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}
	m.Start()

	// Wait for check cycle (Failed)
	// Poll for result instead of fixed sleep
	var active []db.MonitorOutage
	var err error
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		active, err = s.GetActiveOutages()
		if err == nil && len(active) > 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if err != nil {
		t.Fatalf("GetActiveOutages failed: %v", err)
	}

	if len(active) == 0 {
		// Debug: check events
		events, _ := s.GetMonitorEvents("m-fail", 10)
		t.Logf("Events: %+v", events)
		t.Fatal("Expected active outage within 5s, got 0")
	}
	if active[0].MonitorID != "m-fail" {
		t.Errorf("Expected outage for m-fail, got %s", active[0].MonitorID)
	}

	// 3. Verify Legacy Event Created (Regression Test)
	events, err := s.GetMonitorEvents("m-fail", 10)
	if err != nil {
		t.Fatalf("GetMonitorEvents failed: %v", err)
	}
	if len(events) == 0 {
		t.Error("Expected legacy monitor event, got 0")
	} else {
		if events[0].Type != "down" {
			t.Errorf("Expected legacy event type 'down', got '%s'", events[0].Type)
		}
	}

	m.Stop()
}

func TestManager_LatencyThreshold(t *testing.T) {
	m, _ := newTestManager(t)

	if m.GetLatencyThreshold() != 1000 {
		t.Errorf("Expected default 1000, got %d", m.GetLatencyThreshold())
	}

	m.SetLatencyThreshold(500)
	if m.GetLatencyThreshold() != 500 {
		t.Errorf("Expected 500, got %d", m.GetLatencyThreshold())
	}
}

func TestManager_IsGroupInMaintenance(t *testing.T) {
	m, s := newTestManager(t)

	// 1. Create Maintenance Window (Active)
	startTime := time.Now().Add(-1 * time.Hour)
	endTime := time.Now().Add(1 * time.Hour)

	incident := db.Incident{
		ID:             "inc-maint",
		Title:          "Maintenance",
		Type:           "maintenance",
		Status:         "scheduled",
		StartTime:      startTime,
		EndTime:        &endTime,
		AffectedGroups: `["g1"]`,
	}
	// Direct insert or use Store method
	if err := s.CreateIncident(incident); err != nil {
		t.Fatalf("Failed to create maintenance: %v", err)
	}

	// 2. Sync Manager (loads maintenance)
	m.Sync()

	// 3. Verify
	if !m.IsGroupInMaintenance("g1") {
		t.Error("Group g1 should be in maintenance")
	}
	if m.IsGroupInMaintenance("g2") {
		t.Error("Group g2 should NOT be in maintenance")
	}

	// 4. Test Future Maintenance
	futureStart := time.Now().Add(1 * time.Hour)
	futureEnd := time.Now().Add(2 * time.Hour)
	incidentFuture := db.Incident{
		ID:             "inc-future",
		Title:          "Future",
		Type:           "maintenance",
		Status:         "scheduled",
		StartTime:      futureStart,
		EndTime:        &futureEnd,
		AffectedGroups: `["g2"]`,
	}
	if err := s.CreateIncident(incidentFuture); err != nil {
		t.Fatalf("Failed to create future maintenance: %v", err)
	}
	m.Sync()

	if m.IsGroupInMaintenance("g2") {
		t.Error("Group g2 should not be in maintenance (future)")
	}
}
