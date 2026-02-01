package db

import (
	"testing"
	"time"
)

// TestMultiDB_GroupCRUD tests group operations on all database backends
func TestMultiDB_GroupCRUD(t *testing.T) {
	RunTestWithBothDBs(t, "GroupCRUD", func(t *testing.T, s *Store) {
		// Create
		g := Group{ID: "g1", Name: "Group 1", CreatedAt: time.Now()}
		if err := s.CreateGroup(g); err != nil {
			t.Fatalf("CreateGroup failed: %v", err)
		}

		// Read
		groups, err := s.GetGroups()
		if err != nil {
			t.Fatalf("GetGroups failed: %v", err)
		}
		if len(groups) != 1 {
			t.Errorf("Expected 1 group, got %d", len(groups))
		}
		if groups[0].Name != "Group 1" {
			t.Errorf("Expected group name 'Group 1', got '%s'", groups[0].Name)
		}

		// Update
		if err := s.UpdateGroup("g1", "Updated Group"); err != nil {
			t.Fatalf("UpdateGroup failed: %v", err)
		}
		groups, _ = s.GetGroups()
		if groups[0].Name != "Updated Group" {
			t.Errorf("Expected 'Updated Group', got '%s'", groups[0].Name)
		}

		// Delete
		if err := s.DeleteGroup("g1"); err != nil {
			t.Fatalf("DeleteGroup failed: %v", err)
		}
		groups, _ = s.GetGroups()
		if len(groups) != 0 {
			t.Errorf("Expected 0 groups after delete, got %d", len(groups))
		}
	})
}

// TestMultiDB_MonitorCRUD tests monitor operations on all database backends
func TestMultiDB_MonitorCRUD(t *testing.T) {
	RunTestWithBothDBs(t, "MonitorCRUD", func(t *testing.T, s *Store) {
		// Create group first
		g := Group{ID: "g1", Name: "Group 1"}
		if err := s.CreateGroup(g); err != nil {
			t.Fatalf("CreateGroup failed: %v", err)
		}

		// Create monitor
		m := Monitor{
			ID:       "m1",
			GroupID:  "g1",
			Name:     "Test Monitor",
			URL:      "https://example.com",
			Active:   true,
			Interval: 60,
		}
		if err := s.CreateMonitor(m); err != nil {
			t.Fatalf("CreateMonitor failed: %v", err)
		}

		// Read
		monitors, err := s.GetMonitors()
		if err != nil {
			t.Fatalf("GetMonitors failed: %v", err)
		}
		if len(monitors) != 1 {
			t.Errorf("Expected 1 monitor, got %d", len(monitors))
		}
		if monitors[0].Name != "Test Monitor" {
			t.Errorf("Expected 'Test Monitor', got '%s'", monitors[0].Name)
		}

		// Update
		if err := s.UpdateMonitor("m1", "Updated Monitor", "https://updated.com", 120); err != nil {
			t.Fatalf("UpdateMonitor failed: %v", err)
		}
		monitors, _ = s.GetMonitors()
		if monitors[0].Name != "Updated Monitor" {
			t.Errorf("Expected 'Updated Monitor', got '%s'", monitors[0].Name)
		}

		// Delete
		if err := s.DeleteMonitor("m1"); err != nil {
			t.Fatalf("DeleteMonitor failed: %v", err)
		}
		monitors, _ = s.GetMonitors()
		if len(monitors) != 0 {
			t.Errorf("Expected 0 monitors after delete, got %d", len(monitors))
		}
	})
}

// TestMultiDB_UserCRUD tests user operations on all database backends
func TestMultiDB_UserCRUD(t *testing.T) {
	RunTestWithBothDBs(t, "UserCRUD", func(t *testing.T, s *Store) {
		// Create user
		if err := s.CreateUser("testuser", "password123", "UTC"); err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}

		// Check user exists
		hasUsers, err := s.HasUsers()
		if err != nil {
			t.Fatalf("HasUsers failed: %v", err)
		}
		if !hasUsers {
			t.Error("Expected HasUsers to return true")
		}

		// Authenticate
		user, err := s.Authenticate("testuser", "password123")
		if err != nil {
			t.Fatalf("Authenticate failed: %v", err)
		}
		if user.Username != "testuser" {
			t.Errorf("Expected username 'testuser', got '%s'", user.Username)
		}

		// Update user
		if err := s.UpdateUser(user.ID, "", "America/New_York"); err != nil {
			t.Fatalf("UpdateUser failed: %v", err)
		}
		updatedUser, _ := s.GetUser(user.ID)
		if updatedUser.Timezone != "America/New_York" {
			t.Errorf("Expected timezone 'America/New_York', got '%s'", updatedUser.Timezone)
		}
	})
}

// TestMultiDB_Sessions tests session operations on all database backends
func TestMultiDB_Sessions(t *testing.T) {
	RunTestWithBothDBs(t, "Sessions", func(t *testing.T, s *Store) {
		// Create user first
		if err := s.CreateUser("sessionuser", "password123", "UTC"); err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}
		user, _ := s.Authenticate("sessionuser", "password123")

		// Create session
		token := "test-session-token-12345"
		expiresAt := time.Now().Add(24 * time.Hour)
		if err := s.CreateSession(user.ID, token, expiresAt); err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}

		// Get session
		session, err := s.GetSession(token)
		if err != nil {
			t.Fatalf("GetSession failed: %v", err)
		}
		if session == nil {
			t.Fatal("Expected session, got nil")
		}
		if session.UserID != user.ID {
			t.Errorf("Expected user ID %d, got %d", user.ID, session.UserID)
		}

		// Delete session
		if err := s.DeleteSession(token); err != nil {
			t.Fatalf("DeleteSession failed: %v", err)
		}
		session, _ = s.GetSession(token)
		if session != nil {
			t.Error("Expected nil session after delete")
		}
	})
}

// TestMultiDB_MonitorChecks tests monitor check operations on all database backends
func TestMultiDB_MonitorChecks(t *testing.T) {
	RunTestWithBothDBs(t, "MonitorChecks", func(t *testing.T, s *Store) {
		// Setup
		g := Group{ID: "g1", Name: "Group 1"}
		if err := s.CreateGroup(g); err != nil {
			t.Fatalf("CreateGroup failed: %v", err)
		}
		m := Monitor{ID: "m1", GroupID: "g1", Name: "Test", URL: "https://example.com", Active: true, Interval: 60}
		if err := s.CreateMonitor(m); err != nil {
			t.Fatalf("CreateMonitor failed: %v", err)
		}

		// Batch insert checks
		checks := []CheckResult{
			{MonitorID: "m1", Status: "up", Latency: 100, Timestamp: time.Now(), StatusCode: 200},
			{MonitorID: "m1", Status: "up", Latency: 150, Timestamp: time.Now(), StatusCode: 200},
			{MonitorID: "m1", Status: "down", Latency: 0, Timestamp: time.Now(), StatusCode: 500},
		}
		if err := s.BatchInsertChecks(checks); err != nil {
			t.Fatalf("BatchInsertChecks failed: %v", err)
		}

		// Get checks
		results, err := s.GetMonitorChecks("m1", 10)
		if err != nil {
			t.Fatalf("GetMonitorChecks failed: %v", err)
		}
		if len(results) != 3 {
			t.Errorf("Expected 3 checks, got %d", len(results))
		}

		// Get uptime stats
		up24, up7, up30, err := s.GetUptimeStats("m1")
		if err != nil {
			t.Fatalf("GetUptimeStats failed: %v", err)
		}
		// With 2 up and 1 down, expect ~66.67% uptime
		if up24 < 60 || up24 > 70 {
			t.Errorf("Expected ~66.67%% uptime, got %.2f%%", up24)
		}
		_ = up7
		_ = up30
	})
}

// TestMultiDB_Incidents tests incident operations on all database backends
func TestMultiDB_Incidents(t *testing.T) {
	RunTestWithBothDBs(t, "Incidents", func(t *testing.T, s *Store) {
		// Create incident
		incident := Incident{
			ID:             "inc-1",
			Title:          "Test Incident",
			Description:    "Test description",
			Type:           "incident",
			Severity:       "minor",
			Status:         "investigating",
			StartTime:      time.Now(),
			AffectedGroups: "[]",
		}
		if err := s.CreateIncident(incident); err != nil {
			t.Fatalf("CreateIncident failed: %v", err)
		}

		// Get incidents
		incidents, err := s.GetIncidents(time.Now().Add(-24 * time.Hour))
		if err != nil {
			t.Fatalf("GetIncidents failed: %v", err)
		}
		if len(incidents) != 1 {
			t.Errorf("Expected 1 incident, got %d", len(incidents))
		}

		// Update incident
		incident.Status = "resolved"
		if err := s.UpdateIncident(incident); err != nil {
			t.Fatalf("UpdateIncident failed: %v", err)
		}

		// Delete incident
		if err := s.DeleteIncident("inc-1"); err != nil {
			t.Fatalf("DeleteIncident failed: %v", err)
		}
		incidents, _ = s.GetIncidents(time.Now().Add(-24 * time.Hour))
		if len(incidents) != 0 {
			t.Errorf("Expected 0 incidents after delete, got %d", len(incidents))
		}
	})
}

// TestMultiDB_APIKeys tests API key operations on all database backends
func TestMultiDB_APIKeys(t *testing.T) {
	RunTestWithBothDBs(t, "APIKeys", func(t *testing.T, s *Store) {
		// Create API key
		key, err := s.CreateAPIKey("Test Key")
		if err != nil {
			t.Fatalf("CreateAPIKey failed: %v", err)
		}
		if len(key) == 0 {
			t.Fatal("Expected non-empty key")
		}

		// Validate key
		valid, err := s.ValidateAPIKey(key)
		if err != nil {
			t.Fatalf("ValidateAPIKey failed: %v", err)
		}
		if !valid {
			t.Error("Expected key to be valid")
		}

		// Invalid key should fail
		valid, _ = s.ValidateAPIKey("sk_live_INVALID")
		if valid {
			t.Error("Expected invalid key to be rejected")
		}

		// List keys
		keys, err := s.ListAPIKeys()
		if err != nil {
			t.Fatalf("ListAPIKeys failed: %v", err)
		}
		if len(keys) != 1 {
			t.Errorf("Expected 1 key, got %d", len(keys))
		}

		// Delete key
		if err := s.DeleteAPIKey(keys[0].ID); err != nil {
			t.Fatalf("DeleteAPIKey failed: %v", err)
		}
	})
}

// TestMultiDB_Settings tests settings operations on all database backends
func TestMultiDB_Settings(t *testing.T) {
	RunTestWithBothDBs(t, "Settings", func(t *testing.T, s *Store) {
		// Set setting
		if err := s.SetSetting("test_key", "test_value"); err != nil {
			t.Fatalf("SetSetting failed: %v", err)
		}

		// Get setting
		value, err := s.GetSetting("test_key")
		if err != nil {
			t.Fatalf("GetSetting failed: %v", err)
		}
		if value != "test_value" {
			t.Errorf("Expected 'test_value', got '%s'", value)
		}

		// Update setting
		if err := s.SetSetting("test_key", "updated_value"); err != nil {
			t.Fatalf("SetSetting (update) failed: %v", err)
		}
		value, _ = s.GetSetting("test_key")
		if value != "updated_value" {
			t.Errorf("Expected 'updated_value', got '%s'", value)
		}
	})
}

// TestMultiDB_NotificationChannels tests notification channel operations on all database backends
func TestMultiDB_NotificationChannels(t *testing.T) {
	RunTestWithBothDBs(t, "NotificationChannels", func(t *testing.T, s *Store) {
		// Create channel
		channel := NotificationChannel{
			ID:        "nc-1",
			Type:      "slack",
			Name:      "Test Slack",
			Config:    `{"webhook_url": "https://hooks.slack.com/test"}`,
			Enabled:   true,
			CreatedAt: time.Now(),
		}
		if err := s.CreateNotificationChannel(channel); err != nil {
			t.Fatalf("CreateNotificationChannel failed: %v", err)
		}

		// Get channels
		channels, err := s.GetNotificationChannels()
		if err != nil {
			t.Fatalf("GetNotificationChannels failed: %v", err)
		}
		if len(channels) != 1 {
			t.Errorf("Expected 1 channel, got %d", len(channels))
		}

		// Delete channel
		if err := s.DeleteNotificationChannel("nc-1"); err != nil {
			t.Fatalf("DeleteNotificationChannel failed: %v", err)
		}
		channels, _ = s.GetNotificationChannels()
		if len(channels) != 0 {
			t.Errorf("Expected 0 channels after delete, got %d", len(channels))
		}
	})
}

// TestMultiDB_StatusPages tests status page operations on all database backends
func TestMultiDB_StatusPages(t *testing.T) {
	RunTestWithBothDBs(t, "StatusPages", func(t *testing.T, s *Store) {
		// Upsert status page
		if err := s.UpsertStatusPage("test-page", "Test Status Page", nil, true); err != nil {
			t.Fatalf("UpsertStatusPage failed: %v", err)
		}

		// Get by slug
		page, err := s.GetStatusPageBySlug("test-page")
		if err != nil {
			t.Fatalf("GetStatusPageBySlug failed: %v", err)
		}
		if page == nil {
			t.Fatal("Expected page, got nil")
		}
		if page.Title != "Test Status Page" {
			t.Errorf("Expected title 'Test Status Page', got '%s'", page.Title)
		}

		// Toggle public status
		if err := s.ToggleStatusPage("test-page", false); err != nil {
			t.Fatalf("ToggleStatusPage failed: %v", err)
		}
		page, _ = s.GetStatusPageBySlug("test-page")
		if page.Public {
			t.Error("Expected page to be private after toggle")
		}

		// Get all pages
		pages, err := s.GetStatusPages()
		if err != nil {
			t.Fatalf("GetStatusPages failed: %v", err)
		}
		// Should have at least our test page (plus maybe the seeded 'all' page)
		found := false
		for _, p := range pages {
			if p.Slug == "test-page" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected to find 'test-page' in status pages")
		}
	})
}

// TestMultiDB_Outages tests outage operations on all database backends
func TestMultiDB_Outages(t *testing.T) {
	RunTestWithBothDBs(t, "Outages", func(t *testing.T, s *Store) {
		// Setup
		g := Group{ID: "g1", Name: "Group 1"}
		if err := s.CreateGroup(g); err != nil {
			t.Fatalf("CreateGroup failed: %v", err)
		}
		m := Monitor{ID: "m1", GroupID: "g1", Name: "Test", URL: "https://example.com", Active: true, Interval: 60}
		if err := s.CreateMonitor(m); err != nil {
			t.Fatalf("CreateMonitor failed: %v", err)
		}

		// Create outage
		if err := s.CreateOutage("m1", "down", "Server unreachable"); err != nil {
			t.Fatalf("CreateOutage failed: %v", err)
		}

		// Get active outages
		outages, err := s.GetActiveOutages()
		if err != nil {
			t.Fatalf("GetActiveOutages failed: %v", err)
		}
		if len(outages) != 1 {
			t.Errorf("Expected 1 active outage, got %d", len(outages))
		}

		// Close outage
		if err := s.CloseOutage("m1"); err != nil {
			t.Fatalf("CloseOutage failed: %v", err)
		}
		outages, _ = s.GetActiveOutages()
		if len(outages) != 0 {
			t.Errorf("Expected 0 active outages after close, got %d", len(outages))
		}

		// Get resolved outages
		resolved, err := s.GetResolvedOutages(time.Now().Add(-1 * time.Hour))
		if err != nil {
			t.Fatalf("GetResolvedOutages failed: %v", err)
		}
		if len(resolved) != 1 {
			t.Errorf("Expected 1 resolved outage, got %d", len(resolved))
		}
	})
}

// TestMultiDB_Events tests event operations on all database backends
func TestMultiDB_Events(t *testing.T) {
	RunTestWithBothDBs(t, "Events", func(t *testing.T, s *Store) {
		// Setup
		g := Group{ID: "g1", Name: "Group 1"}
		if err := s.CreateGroup(g); err != nil {
			t.Fatalf("CreateGroup failed: %v", err)
		}
		m := Monitor{ID: "m1", GroupID: "g1", Name: "Test", URL: "https://example.com", Active: true, Interval: 60}
		if err := s.CreateMonitor(m); err != nil {
			t.Fatalf("CreateMonitor failed: %v", err)
		}

		// Create events
		if err := s.CreateEvent("m1", "down", "Server went down"); err != nil {
			t.Fatalf("CreateEvent failed: %v", err)
		}
		if err := s.CreateEvent("m1", "up", "Server recovered"); err != nil {
			t.Fatalf("CreateEvent failed: %v", err)
		}

		// Get monitor events
		events, err := s.GetMonitorEvents("m1", 10)
		if err != nil {
			t.Fatalf("GetMonitorEvents failed: %v", err)
		}
		if len(events) != 2 {
			t.Errorf("Expected 2 events, got %d", len(events))
		}

		// Get system events
		sysEvents, err := s.GetSystemEvents(10)
		if err != nil {
			t.Fatalf("GetSystemEvents failed: %v", err)
		}
		if len(sysEvents) != 2 {
			t.Errorf("Expected 2 system events, got %d", len(sysEvents))
		}
	})
}

// TestMultiDB_SystemStats tests system stats on all database backends
func TestMultiDB_SystemStats(t *testing.T) {
	RunTestWithBothDBs(t, "SystemStats", func(t *testing.T, s *Store) {
		// Setup
		g := Group{ID: "g1", Name: "Group 1"}
		if err := s.CreateGroup(g); err != nil {
			t.Fatalf("CreateGroup failed: %v", err)
		}
		m := Monitor{ID: "m1", GroupID: "g1", Name: "Test", URL: "https://example.com", Active: true, Interval: 60}
		if err := s.CreateMonitor(m); err != nil {
			t.Fatalf("CreateMonitor failed: %v", err)
		}

		// Get stats
		stats, err := s.GetSystemStats()
		if err != nil {
			t.Fatalf("GetSystemStats failed: %v", err)
		}
		if stats.TotalMonitors != 1 {
			t.Errorf("Expected 1 total monitor, got %d", stats.TotalMonitors)
		}
		if stats.ActiveMonitors != 1 {
			t.Errorf("Expected 1 active monitor, got %d", stats.ActiveMonitors)
		}
		if stats.TotalGroups != 1 {
			t.Errorf("Expected 1 group, got %d", stats.TotalGroups)
		}
	})
}

// TestMultiDB_DBSize tests database size query on all database backends
func TestMultiDB_DBSize(t *testing.T) {
	RunTestWithBothDBs(t, "DBSize", func(t *testing.T, s *Store) {
		size, err := s.GetDBSize()
		if err != nil {
			t.Fatalf("GetDBSize failed: %v", err)
		}
		if size <= 0 {
			t.Errorf("Expected positive DB size, got %d", size)
		}
	})
}
