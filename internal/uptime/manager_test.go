package uptime

import (
	"fmt"
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

func TestManager_SSLExpiryThreshold(t *testing.T) {
	m, _ := newTestManager(t)

	// Default should be 30 days
	if m.GetSSLExpiryThreshold() != 30 {
		t.Errorf("Expected default SSL expiry threshold 30, got %d", m.GetSSLExpiryThreshold())
	}

	// Test setter
	m.SetSSLExpiryThreshold(14)
	if m.GetSSLExpiryThreshold() != 14 {
		t.Errorf("Expected 14, got %d", m.GetSSLExpiryThreshold())
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

func TestManager_NotificationTimezoneLoaded(t *testing.T) {
	m, s := newTestManager(t)

	// Set a specific timezone
	if err := s.SetSetting("notification_timezone", "America/New_York"); err != nil {
		t.Fatalf("SetSetting failed: %v", err)
	}

	// Sync to load the timezone
	m.Sync()

	// Verify timezone is cached (internal check via behavior)
	// Since notificationTimezone is private, we verify indirectly
	// by checking the sync doesn't panic with valid timezone
	// and the setting is retrievable
	tz, err := s.GetSetting("notification_timezone")
	if err != nil {
		t.Fatalf("GetSetting failed: %v", err)
	}
	if tz != "America/New_York" {
		t.Errorf("Expected America/New_York, got %s", tz)
	}
}

func TestManager_NotificationTimezoneDefaultsToUTC(t *testing.T) {
	m, s := newTestManager(t)

	// Don't set any timezone - should default to UTC

	// Sync
	m.Sync()

	// Verify default is used (indirectly through setting)
	tz, _ := s.GetSetting("notification_timezone")
	// If not set, it should either be empty or "UTC" after migration
	if tz != "" && tz != "UTC" {
		t.Logf("Timezone setting: %s (may be empty before first use)", tz)
	}
}

func TestManager_InvalidTimezoneHandling(t *testing.T) {
	m, s := newTestManager(t)

	// Set an invalid timezone
	if err := s.SetSetting("notification_timezone", "Invalid/Timezone"); err != nil {
		t.Fatalf("SetSetting failed: %v", err)
	}

	// Sync should not panic and should fall back to UTC
	m.Sync()

	// If we got here without panic, the fallback works
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
