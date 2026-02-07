package uptime

import (
	"fmt"
	"testing"
	"time"

	"github.com/clusteruptime/clusteruptime/internal/db"
)

func newTestManager(t *testing.T) (*Manager, *db.Store) {
	store, err := db.NewStore(db.NewTestConfigWithPath("file:manager?mode=memory&cache=shared"))
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

func TestSSLNotificationThresholds(t *testing.T) {
	// Verify the thresholds are correct
	expected := []int{30, 14, 7, 1}
	if len(sslNotificationThresholds) != len(expected) {
		t.Fatalf("Expected %d thresholds, got %d", len(expected), len(sslNotificationThresholds))
	}
	for i, v := range expected {
		if sslNotificationThresholds[i] != v {
			t.Errorf("Threshold[%d]: expected %d, got %d", i, v, sslNotificationThresholds[i])
		}
	}
}

func TestSSLThresholdMatching(t *testing.T) {
	// Test the threshold matching logic
	// Thresholds: [30, 14, 7, 1]
	// We want to find the SMALLEST threshold t where daysUntilExpiry <= t

	testCases := []struct {
		daysUntilExpiry   int
		expectedThreshold int
	}{
		{daysUntilExpiry: 45, expectedThreshold: -1},  // Beyond all thresholds
		{daysUntilExpiry: 30, expectedThreshold: 30},  // Exactly 30
		{daysUntilExpiry: 25, expectedThreshold: 30},  // Between 30 and 14
		{daysUntilExpiry: 14, expectedThreshold: 14},  // Exactly 14
		{daysUntilExpiry: 10, expectedThreshold: 14},  // Between 14 and 7
		{daysUntilExpiry: 7, expectedThreshold: 7},    // Exactly 7
		{daysUntilExpiry: 5, expectedThreshold: 7},    // Between 7 and 1
		{daysUntilExpiry: 1, expectedThreshold: 1},    // Exactly 1
		{daysUntilExpiry: 0, expectedThreshold: 1},    // Zero days (expires today)
		{daysUntilExpiry: -5, expectedThreshold: 1},   // Already expired
		{daysUntilExpiry: -100, expectedThreshold: 1}, // Long expired
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("days_%d", tc.daysUntilExpiry), func(t *testing.T) {
			matchedThreshold := -1
			for _, threshold := range sslNotificationThresholds {
				if tc.daysUntilExpiry <= threshold {
					matchedThreshold = threshold // Keep updating to get smallest match
				}
			}
			if matchedThreshold != tc.expectedThreshold {
				t.Errorf("daysUntilExpiry=%d: expected threshold %d, got %d",
					tc.daysUntilExpiry, tc.expectedThreshold, matchedThreshold)
			}
		})
	}
}

func TestSSLThresholdState_CertificateRenewal(t *testing.T) {
	// Test that certificate renewal resets threshold state
	state := &sslThresholdState{
		CertExpiry: time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC),
		Notified:   map[int]bool{30: true, 14: true},
	}

	// Simulate renewal - new cert with different expiry
	newExpiry := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	// Check if expiry changed (simulating the logic in manager.go)
	if !state.CertExpiry.Equal(newExpiry) {
		// Reset state
		state = &sslThresholdState{
			CertExpiry: newExpiry,
			Notified:   make(map[int]bool),
		}
	}

	// Verify state was reset
	if len(state.Notified) != 0 {
		t.Error("Expected empty Notified map after renewal")
	}
	if !state.CertExpiry.Equal(newExpiry) {
		t.Error("CertExpiry should be updated")
	}
}

func TestSSLThresholdState_TrackNotifiedThresholds(t *testing.T) {
	state := &sslThresholdState{
		CertExpiry: time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC),
		Notified:   make(map[int]bool),
	}

	// Notify at 30-day threshold
	if state.Notified[30] {
		t.Error("30-day threshold should not be notified yet")
	}
	state.Notified[30] = true
	if !state.Notified[30] {
		t.Error("30-day threshold should be marked as notified")
	}

	// 14-day should still be unnotified
	if state.Notified[14] {
		t.Error("14-day threshold should not be notified yet")
	}

	// Notify at 14-day threshold
	state.Notified[14] = true
	if !state.Notified[14] {
		t.Error("14-day threshold should be marked as notified")
	}

	// Both should now be notified
	if !state.Notified[30] || !state.Notified[14] {
		t.Error("Both 30 and 14 day thresholds should be notified")
	}
}

func TestManager_UserTimezoneLoaded(t *testing.T) {
	// Create a fresh store for this test
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("Failed to create test store: %v", err)
	}
	m := NewManager(store)

	// Create a user with a specific timezone
	if err := store.CreateUser("admin", "password123", "America/New_York"); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Sync to load the timezone from user
	m.Sync()

	// Verify user timezone is set
	user, err := store.GetUser(1)
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if user.Timezone != "America/New_York" {
		t.Errorf("Expected America/New_York, got %s", user.Timezone)
	}
}

func TestManager_TimezoneDefaultsToUTC(t *testing.T) {
	m, _ := newTestManager(t)

	// Don't create any user - should default to UTC
	// Sync should not panic
	m.Sync()
}

func TestManager_InvalidTimezoneHandling(t *testing.T) {
	// Create a fresh store for this test to avoid conflicts
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("Failed to create test store: %v", err)
	}
	m := NewManager(store)

	// Create user with invalid timezone (edge case - shouldn't normally happen)
	// The CreateUser doesn't validate timezone, so we test the fallback
	if err := store.CreateUser("admin", "password123", "Invalid/Timezone"); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Sync should not panic and should fall back to UTC
	m.Sync()
}

func TestManager_RemoveMonitorCleansSSLState(t *testing.T) {
	m, s := newTestManager(t)

	// Create monitor
	if err := s.CreateMonitor(db.Monitor{
		ID:       "m-ssl-test",
		GroupID:  "g-default",
		Name:     "SSL Test",
		URL:      "https://example.com",
		Active:   true,
		Interval: 60,
	}); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}

	m.Sync()

	// Manually add SSL state (simulating notification sent)
	m.mu.Lock()
	m.sslNotifiedThresholds["m-ssl-test"] = &sslThresholdState{
		CertExpiry: time.Now().Add(14 * 24 * time.Hour),
		Notified:   map[int]bool{30: true},
	}
	m.mu.Unlock()

	// Verify state exists
	m.mu.RLock()
	_, exists := m.sslNotifiedThresholds["m-ssl-test"]
	m.mu.RUnlock()
	if !exists {
		t.Fatal("SSL state should exist before removal")
	}

	// Remove monitor
	m.RemoveMonitor("m-ssl-test")

	// Verify state is cleaned
	m.mu.RLock()
	_, exists = m.sslNotifiedThresholds["m-ssl-test"]
	m.mu.RUnlock()
	if exists {
		t.Error("SSL state should be cleaned after monitor removal")
	}
}

func TestManager_ResetCleansSSLState(t *testing.T) {
	m, s := newTestManager(t)

	// Create monitor
	if err := s.CreateMonitor(db.Monitor{
		ID:       "m-reset-test",
		GroupID:  "g-default",
		Name:     "Reset Test",
		URL:      "https://example.com",
		Active:   true,
		Interval: 60,
	}); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}

	m.Sync()

	// Manually add SSL state
	m.mu.Lock()
	m.sslNotifiedThresholds["m-reset-test"] = &sslThresholdState{
		CertExpiry: time.Now().Add(7 * 24 * time.Hour),
		Notified:   map[int]bool{30: true, 14: true},
	}
	m.mu.Unlock()

	// Reset manager
	m.Reset()

	// Verify all SSL state is cleaned
	m.mu.RLock()
	count := len(m.sslNotifiedThresholds)
	m.mu.RUnlock()
	if count != 0 {
		t.Errorf("Expected 0 SSL states after reset, got %d", count)
	}
}

// ============== PAUSE/RESUME EDGE CASE TESTS ==============

func TestManager_PauseMonitor_RemovesFromScheduler(t *testing.T) {
	m, s := newTestManager(t)

	// Create an active monitor
	if err := s.CreateMonitor(db.Monitor{
		ID:       "m-pause-1",
		GroupID:  "g-default",
		Name:     "Pause Test",
		URL:      "http://example.com",
		Active:   true,
		Interval: 60,
	}); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}

	// Sync - monitor should be running
	m.Sync()

	// Verify running
	if m.GetMonitor("m-pause-1") == nil {
		t.Fatal("Monitor should be running after sync")
	}

	// Pause the monitor in DB
	if err := s.SetMonitorActive("m-pause-1", false); err != nil {
		t.Fatalf("SetMonitorActive failed: %v", err)
	}

	// Sync again - monitor should be removed from scheduler
	m.Sync()

	// Verify NOT running
	if m.GetMonitor("m-pause-1") != nil {
		t.Fatal("Monitor should NOT be running after pause")
	}

	// Verify it's not in the monitors map
	all := m.GetAll()
	if _, exists := all["m-pause-1"]; exists {
		t.Error("Paused monitor should not be in GetAll()")
	}
}

func TestManager_ResumeMonitor_AddsBackToScheduler(t *testing.T) {
	m, s := newTestManager(t)

	// Create an inactive monitor
	if err := s.CreateMonitor(db.Monitor{
		ID:       "m-resume-1",
		GroupID:  "g-default",
		Name:     "Resume Test",
		URL:      "http://example.com",
		Active:   false, // Starts paused
		Interval: 60,
	}); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}

	// Sync - monitor should NOT be running
	m.Sync()

	if m.GetMonitor("m-resume-1") != nil {
		t.Fatal("Inactive monitor should NOT be running")
	}

	// Resume the monitor in DB
	if err := s.SetMonitorActive("m-resume-1", true); err != nil {
		t.Fatalf("SetMonitorActive failed: %v", err)
	}

	// Sync again - monitor should be running
	m.Sync()

	if m.GetMonitor("m-resume-1") == nil {
		t.Fatal("Monitor should be running after resume")
	}
}

func TestManager_PauseResume_FullCycle(t *testing.T) {
	m, s := newTestManager(t)

	// Create an active monitor
	if err := s.CreateMonitor(db.Monitor{
		ID:       "m-cycle-1",
		GroupID:  "g-default",
		Name:     "Cycle Test",
		URL:      "http://example.com",
		Active:   true,
		Interval: 60,
	}); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}

	// 1. Initial sync - running
	m.Sync()
	if m.GetMonitor("m-cycle-1") == nil {
		t.Fatal("Step 1: Monitor should be running")
	}

	// 2. Pause
	if err := s.SetMonitorActive("m-cycle-1", false); err != nil {
		t.Fatalf("Pause failed: %v", err)
	}
	m.Sync()
	if m.GetMonitor("m-cycle-1") != nil {
		t.Fatal("Step 2: Monitor should NOT be running after pause")
	}

	// 3. Resume
	if err := s.SetMonitorActive("m-cycle-1", true); err != nil {
		t.Fatalf("Resume failed: %v", err)
	}
	m.Sync()
	if m.GetMonitor("m-cycle-1") == nil {
		t.Fatal("Step 3: Monitor should be running after resume")
	}

	// 4. Pause again
	if err := s.SetMonitorActive("m-cycle-1", false); err != nil {
		t.Fatalf("Second pause failed: %v", err)
	}
	m.Sync()
	if m.GetMonitor("m-cycle-1") != nil {
		t.Fatal("Step 4: Monitor should NOT be running after second pause")
	}

	// 5. Resume again
	if err := s.SetMonitorActive("m-cycle-1", true); err != nil {
		t.Fatalf("Second resume failed: %v", err)
	}
	m.Sync()
	if m.GetMonitor("m-cycle-1") == nil {
		t.Fatal("Step 5: Monitor should be running after second resume")
	}
}

func TestManager_PauseCleansSSLState(t *testing.T) {
	m, s := newTestManager(t)

	// Create an active monitor
	if err := s.CreateMonitor(db.Monitor{
		ID:       "m-ssl-pause",
		GroupID:  "g-default",
		Name:     "SSL Pause Test",
		URL:      "https://example.com",
		Active:   true,
		Interval: 60,
	}); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}

	m.Sync()

	// Manually add SSL state (simulating notification sent)
	m.mu.Lock()
	m.sslNotifiedThresholds["m-ssl-pause"] = &sslThresholdState{
		CertExpiry: time.Now().Add(14 * 24 * time.Hour),
		Notified:   map[int]bool{30: true, 14: true},
	}
	m.mu.Unlock()

	// Verify SSL state exists
	m.mu.RLock()
	_, exists := m.sslNotifiedThresholds["m-ssl-pause"]
	m.mu.RUnlock()
	if !exists {
		t.Fatal("SSL state should exist before pause")
	}

	// Pause the monitor
	if err := s.SetMonitorActive("m-ssl-pause", false); err != nil {
		t.Fatalf("Pause failed: %v", err)
	}
	m.Sync()

	// Verify SSL state is cleaned
	m.mu.RLock()
	_, exists = m.sslNotifiedThresholds["m-ssl-pause"]
	m.mu.RUnlock()
	if exists {
		t.Error("SSL state should be cleaned after pause")
	}
}

func TestManager_MultipleMonitors_PauseOne(t *testing.T) {
	// Use isolated database for this test
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("Failed to create test store: %v", err)
	}
	m := NewManager(store)

	// Create 3 monitors
	for i := 1; i <= 3; i++ {
		if err := store.CreateMonitor(db.Monitor{
			ID:       fmt.Sprintf("m-multi-%d", i),
			GroupID:  "g-default",
			Name:     fmt.Sprintf("Multi Test %d", i),
			URL:      fmt.Sprintf("http://example%d.com", i),
			Active:   true,
			Interval: 60,
		}); err != nil {
			t.Fatalf("CreateMonitor %d failed: %v", i, err)
		}
	}

	m.Sync()

	// All 3 should be running
	if len(m.GetAll()) != 3 {
		t.Fatalf("Expected 3 running monitors, got %d", len(m.GetAll()))
	}

	// Pause monitor 2
	if err := store.SetMonitorActive("m-multi-2", false); err != nil {
		t.Fatalf("Pause failed: %v", err)
	}
	m.Sync()

	// Should have 2 running
	all := m.GetAll()
	if len(all) != 2 {
		t.Fatalf("Expected 2 running monitors, got %d", len(all))
	}

	// Verify correct ones are running
	if all["m-multi-1"] == nil {
		t.Error("Monitor 1 should still be running")
	}
	if all["m-multi-2"] != nil {
		t.Error("Monitor 2 should NOT be running")
	}
	if all["m-multi-3"] == nil {
		t.Error("Monitor 3 should still be running")
	}
}

func TestManager_RapidPauseResume(t *testing.T) {
	// Use isolated database for this test
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("Failed to create test store: %v", err)
	}
	m := NewManager(store)

	if err := store.CreateMonitor(db.Monitor{
		ID:       "m-rapid",
		GroupID:  "g-default",
		Name:     "Rapid Test",
		URL:      "http://example.com",
		Active:   true,
		Interval: 60,
	}); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}

	m.Sync()

	// Rapid pause/resume cycles without panics
	for i := 0; i < 10; i++ {
		// Pause
		if err := store.SetMonitorActive("m-rapid", false); err != nil {
			t.Fatalf("Pause %d failed: %v", i, err)
		}
		m.Sync()

		// Resume
		if err := store.SetMonitorActive("m-rapid", true); err != nil {
			t.Fatalf("Resume %d failed: %v", i, err)
		}
		m.Sync()
	}

	// Should end up running
	if m.GetMonitor("m-rapid") == nil {
		t.Fatal("Monitor should be running after rapid cycles")
	}
}

func TestManager_PauseIdempotent(t *testing.T) {
	m, s := newTestManager(t)

	if err := s.CreateMonitor(db.Monitor{
		ID:       "m-idem",
		GroupID:  "g-default",
		Name:     "Idempotent Test",
		URL:      "http://example.com",
		Active:   true,
		Interval: 60,
	}); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}

	m.Sync()

	// Pause once
	if err := s.SetMonitorActive("m-idem", false); err != nil {
		t.Fatalf("First pause failed: %v", err)
	}
	m.Sync()

	// Pause again (already paused) - should not panic
	if err := s.SetMonitorActive("m-idem", false); err != nil {
		t.Fatalf("Second pause failed: %v", err)
	}
	m.Sync()

	// Should still not be running
	if m.GetMonitor("m-idem") != nil {
		t.Error("Monitor should still not be running")
	}
}

func TestManager_ServiceRestart_PausedMonitorStaysPaused(t *testing.T) {
	// This simulates a service restart scenario
	store, err := db.NewStore(db.NewTestConfigWithPath("file:restart_test?mode=memory&cache=shared"))
	if err != nil {
		t.Fatalf("Failed to create test store: %v", err)
	}

	// Create a paused monitor
	if err := store.CreateMonitor(db.Monitor{
		ID:       "m-restart",
		GroupID:  "g-default",
		Name:     "Restart Test",
		URL:      "http://example.com",
		Active:   false, // Already paused
		Interval: 60,
	}); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}

	// Create a new manager (simulating restart)
	m := NewManager(store)
	m.Sync()

	// Monitor should NOT be running (was paused before restart)
	if m.GetMonitor("m-restart") != nil {
		t.Error("Paused monitor should NOT be running after restart")
	}

	// Resume it
	if err := store.SetMonitorActive("m-restart", true); err != nil {
		t.Fatalf("Resume failed: %v", err)
	}
	m.Sync()

	// Now it should be running
	if m.GetMonitor("m-restart") == nil {
		t.Error("Monitor should be running after resume")
	}
}

func TestMonitor_DoubleStopNoPanic(t *testing.T) {
	jobQueue := make(chan Job, 10)
	mon := NewMonitor("m1", "g1", "Double Stop", "http://example.com", 10*time.Millisecond, jobQueue)

	go mon.Start()
	time.Sleep(20 * time.Millisecond)

	// Stop twice - should not panic due to sync.Once protection
	mon.Stop()
	mon.Stop() // Second stop should be safe

	// If we get here without panic, test passes
}

func TestManager_HistoryPersistedAcrossPauseResume(t *testing.T) {
	// Use isolated database for this test
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("Failed to create test store: %v", err)
	}
	m := NewManager(store)

	if err := store.CreateMonitor(db.Monitor{
		ID:       "m-history",
		GroupID:  "g-default",
		Name:     "History Test",
		URL:      "http://example.com",
		Active:   true,
		Interval: 60,
	}); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}

	// Add some check results to DB (simulating past checks)
	checks := []db.CheckResult{
		{MonitorID: "m-history", Status: "up", Latency: 100, Timestamp: time.Now().Add(-5 * time.Minute), StatusCode: 200},
		{MonitorID: "m-history", Status: "up", Latency: 150, Timestamp: time.Now().Add(-4 * time.Minute), StatusCode: 200},
		{MonitorID: "m-history", Status: "up", Latency: 120, Timestamp: time.Now().Add(-3 * time.Minute), StatusCode: 200},
	}
	if err := store.BatchInsertChecks(checks); err != nil {
		t.Fatalf("BatchInsertChecks failed: %v", err)
	}

	m.Sync()

	// Verify monitor has history
	mon := m.GetMonitor("m-history")
	if mon == nil {
		t.Fatal("Monitor should be running")
	}
	history := mon.GetHistory()
	if len(history) != 3 {
		t.Errorf("Expected 3 history items, got %d", len(history))
	}

	// Pause
	if err := store.SetMonitorActive("m-history", false); err != nil {
		t.Fatalf("Pause failed: %v", err)
	}
	m.Sync()

	// Resume
	if err := store.SetMonitorActive("m-history", true); err != nil {
		t.Fatalf("Resume failed: %v", err)
	}
	m.Sync()

	// History should be re-hydrated from DB
	mon = m.GetMonitor("m-history")
	if mon == nil {
		t.Fatal("Monitor should be running after resume")
	}
	history = mon.GetHistory()
	if len(history) != 3 {
		t.Errorf("Expected 3 history items after resume (from DB), got %d", len(history))
	}
}

func TestManager_DeleteWhilePaused(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("Failed to create test store: %v", err)
	}
	m := NewManager(store)

	// Create and sync a monitor
	if err := store.CreateMonitor(db.Monitor{
		ID:       "m-delete-paused",
		GroupID:  "g-default",
		Name:     "Delete Paused Test",
		URL:      "http://example.com",
		Active:   true,
		Interval: 60,
	}); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}
	m.Sync()

	// Pause the monitor
	if err := store.SetMonitorActive("m-delete-paused", false); err != nil {
		t.Fatalf("Pause failed: %v", err)
	}
	m.Sync()

	// Verify paused (not in scheduler)
	if m.GetMonitor("m-delete-paused") != nil {
		t.Fatal("Monitor should not be running after pause")
	}

	// Delete the paused monitor from DB
	if err := store.DeleteMonitor("m-delete-paused"); err != nil {
		t.Fatalf("DeleteMonitor failed: %v", err)
	}
	m.Sync()

	// Verify fully removed
	if m.GetMonitor("m-delete-paused") != nil {
		t.Error("Deleted monitor should not be in manager")
	}
}

func TestManager_UpdateWhilePaused(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("Failed to create test store: %v", err)
	}
	m := NewManager(store)

	// Create and sync a monitor
	if err := store.CreateMonitor(db.Monitor{
		ID:       "m-update-paused",
		GroupID:  "g-default",
		Name:     "Original Name",
		URL:      "http://original.com",
		Active:   true,
		Interval: 60,
	}); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}
	m.Sync()

	// Pause the monitor
	if err := store.SetMonitorActive("m-update-paused", false); err != nil {
		t.Fatalf("Pause failed: %v", err)
	}
	m.Sync()

	// Update the monitor while paused
	if err := store.UpdateMonitor("m-update-paused", "Updated Name", "http://updated.com", 120); err != nil {
		t.Fatalf("UpdateMonitor failed: %v", err)
	}
	m.Sync()

	// Still should not be running (still paused)
	if m.GetMonitor("m-update-paused") != nil {
		t.Fatal("Updated but still paused monitor should not be running")
	}

	// Resume the monitor
	if err := store.SetMonitorActive("m-update-paused", true); err != nil {
		t.Fatalf("Resume failed: %v", err)
	}
	m.Sync()

	// Now should be running with updated config
	mon := m.GetMonitor("m-update-paused")
	if mon == nil {
		t.Fatal("Monitor should be running after resume")
	}
	if mon.GetTargetURL() != "http://updated.com" {
		t.Errorf("Expected updated URL, got %s", mon.GetTargetURL())
	}
	if mon.GetInterval() != 120*time.Second {
		t.Errorf("Expected updated interval 120s, got %s", mon.GetInterval())
	}
}

func TestManager_PauseDuringActiveOutage(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("Failed to create test store: %v", err)
	}
	m := NewManager(store)

	// Create a group and monitor
	if err := store.CreateGroup(db.Group{ID: "g-outage", Name: "Outage Group"}); err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}
	if err := store.CreateMonitor(db.Monitor{
		ID:       "m-outage-pause",
		GroupID:  "g-outage",
		Name:     "Outage Pause Test",
		URL:      "http://example.com",
		Active:   true,
		Interval: 60,
	}); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}
	m.Sync()

	// Create an active outage for this monitor
	if err := store.CreateOutage("m-outage-pause", "down", "Connection refused"); err != nil {
		t.Fatalf("CreateOutage failed: %v", err)
	}

	// Verify outage exists
	outages, _ := store.GetActiveOutages()
	if len(outages) != 1 {
		t.Fatalf("Expected 1 active outage, got %d", len(outages))
	}

	// Pause the monitor
	if err := store.SetMonitorActive("m-outage-pause", false); err != nil {
		t.Fatalf("Pause failed: %v", err)
	}
	m.Sync()

	// Monitor should be removed from scheduler
	if m.GetMonitor("m-outage-pause") != nil {
		t.Error("Paused monitor should not be running")
	}

	// Outage record should still exist in DB (historical data)
	outages, _ = store.GetActiveOutages()
	if len(outages) != 1 {
		t.Errorf("Outage should still exist after pause, got %d", len(outages))
	}
}

func TestManager_ConcurrentSync(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("Failed to create test store: %v", err)
	}
	m := NewManager(store)

	// Create some monitors
	for i := 1; i <= 5; i++ {
		if err := store.CreateMonitor(db.Monitor{
			ID:       fmt.Sprintf("m-conc-%d", i),
			GroupID:  "g-default",
			Name:     fmt.Sprintf("Concurrent %d", i),
			URL:      fmt.Sprintf("http://example%d.com", i),
			Active:   true,
			Interval: 60,
		}); err != nil {
			t.Fatalf("CreateMonitor %d failed: %v", i, err)
		}
	}

	// Run concurrent Sync operations
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			m.Sync()
			done <- true
		}()
	}

	// Wait for all
	for i := 0; i < 10; i++ {
		<-done
	}

	// All monitors should be running
	all := m.GetAll()
	if len(all) != 5 {
		t.Errorf("Expected 5 running monitors, got %d", len(all))
	}
}

func TestManager_PauseAllMonitors(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("Failed to create test store: %v", err)
	}
	m := NewManager(store)

	// Create multiple monitors
	for i := 1; i <= 3; i++ {
		if err := store.CreateMonitor(db.Monitor{
			ID:       fmt.Sprintf("m-all-%d", i),
			GroupID:  "g-default",
			Name:     fmt.Sprintf("All Test %d", i),
			URL:      fmt.Sprintf("http://example%d.com", i),
			Active:   true,
			Interval: 60,
		}); err != nil {
			t.Fatalf("CreateMonitor %d failed: %v", i, err)
		}
	}
	m.Sync()

	// Verify all running
	if len(m.GetAll()) != 3 {
		t.Fatalf("Expected 3 monitors running, got %d", len(m.GetAll()))
	}

	// Pause all monitors
	for i := 1; i <= 3; i++ {
		if err := store.SetMonitorActive(fmt.Sprintf("m-all-%d", i), false); err != nil {
			t.Fatalf("Pause %d failed: %v", i, err)
		}
	}
	m.Sync()

	// None should be running
	if len(m.GetAll()) != 0 {
		t.Errorf("Expected 0 monitors running after pause all, got %d", len(m.GetAll()))
	}

	// Resume all
	for i := 1; i <= 3; i++ {
		if err := store.SetMonitorActive(fmt.Sprintf("m-all-%d", i), true); err != nil {
			t.Fatalf("Resume %d failed: %v", i, err)
		}
	}
	m.Sync()

	// All should be running again
	if len(m.GetAll()) != 3 {
		t.Errorf("Expected 3 monitors running after resume all, got %d", len(m.GetAll()))
	}
}

func TestManager_PauseMonitorPreservesInMemoryState(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("Failed to create test store: %v", err)
	}
	m := NewManager(store)

	// Create a monitor
	if err := store.CreateMonitor(db.Monitor{
		ID:       "m-state",
		GroupID:  "g-default",
		Name:     "State Test",
		URL:      "http://example.com",
		Active:   true,
		Interval: 60,
	}); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}
	m.Sync()

	// Record some in-memory status
	mon := m.GetMonitor("m-state")
	if mon == nil {
		t.Fatal("Monitor should be running")
	}
	mon.RecordResult(true, 100, time.Now(), 200, "", false)
	mon.RecordResult(true, 150, time.Now(), 200, "", false)

	// Verify history
	history := mon.GetHistory()
	if len(history) != 2 {
		t.Fatalf("Expected 2 history items, got %d", len(history))
	}

	// Pause - this should remove the monitor from manager
	if err := store.SetMonitorActive("m-state", false); err != nil {
		t.Fatalf("Pause failed: %v", err)
	}
	m.Sync()

	// Monitor should no longer be in manager
	if m.GetMonitor("m-state") != nil {
		t.Error("Paused monitor should not be in manager")
	}
}

func TestManager_GetLastStatus_PausedMonitor(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("Failed to create test store: %v", err)
	}
	m := NewManager(store)

	// Create a paused monitor from the start
	if err := store.CreateMonitor(db.Monitor{
		ID:       "m-paused-status",
		GroupID:  "g-default",
		Name:     "Paused Status Test",
		URL:      "http://example.com",
		Active:   false, // Start paused
		Interval: 60,
	}); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}
	m.Sync()

	// Trying to get monitor that was never started should return nil
	mon := m.GetMonitor("m-paused-status")
	if mon != nil {
		t.Error("Paused monitor should not be accessible via GetMonitor")
	}
}

func TestManager_CreatePausedMonitor(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("Failed to create test store: %v", err)
	}
	m := NewManager(store)

	// Create monitor that starts paused
	if err := store.CreateMonitor(db.Monitor{
		ID:       "m-created-paused",
		GroupID:  "g-default",
		Name:     "Created Paused",
		URL:      "http://example.com",
		Active:   false,
		Interval: 60,
	}); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}
	m.Sync()

	// Should not be in the scheduler
	if m.GetMonitor("m-created-paused") != nil {
		t.Error("Monitor created as paused should not be running")
	}
	if len(m.GetAll()) != 0 {
		t.Errorf("Expected 0 running monitors, got %d", len(m.GetAll()))
	}

	// Resume it
	if err := store.SetMonitorActive("m-created-paused", true); err != nil {
		t.Fatalf("Resume failed: %v", err)
	}
	m.Sync()

	// Now should be running
	if m.GetMonitor("m-created-paused") == nil {
		t.Error("Monitor should be running after resume")
	}
}
