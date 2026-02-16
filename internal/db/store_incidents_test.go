package db

import (
	"testing"
	"time"
)

func TestIncidentCRUD(t *testing.T) {
	s := newTestStore(t)

	i := Incident{
		ID:          "inc-1",
		Title:       "Database Down",
		Description: "DB is unresponsive",
		Type:        "incident",
		Severity:    "critical",
		Status:      "investigating",
		StartTime:   time.Now(),
		CreatedAt:   time.Now(),
	}

	// Create
	if err := s.CreateIncident(i); err != nil {
		t.Fatalf("CreateIncident failed: %v", err)
	}

	// Read
	incidents, err := s.GetIncidents(time.Time{})
	if err != nil {
		t.Fatalf("GetIncidents failed: %v", err)
	}

	found := false
	for _, inc := range incidents {
		if inc.ID == "inc-1" {
			found = true
			if inc.Title != "Database Down" {
				t.Error("Title mismatch")
			}
		}
	}
	if !found {
		t.Error("Incident not found")
	}

	// Update
	i.Status = "resolved"
	now := time.Now()
	i.EndTime = &now
	if err := s.UpdateIncident(i); err != nil {
		t.Fatalf("UpdateIncident failed: %v", err)
	}

	// Read again (Should be updated)
	// Note: GetIncidents filters resolved unless time scope is wide?
	// Query: WHERE (status != 'resolved' AND status != 'completed') OR start_time >= ?
	// Since start_time >= zero time, it should return it.
	incidents, _ = s.GetIncidents(time.Time{})

	found = false
	for _, inc := range incidents {
		if inc.ID == "inc-1" {
			found = true
			if inc.Status != "resolved" {
				t.Error("Status not updated")
			}
		}
	}
	if !found {
		t.Error("Incident not found after update")
	}

	// Delete
	if err := s.DeleteIncident("inc-1"); err != nil {
		t.Fatalf("DeleteIncident failed: %v", err)
	}

	// Verify Gone
	incidents, _ = s.GetIncidents(time.Time{})
	for _, inc := range incidents {
		if inc.ID == "inc-1" {
			t.Error("Incident should be deleted")
		}
	}
}

func TestIncidentWithNewFields(t *testing.T) {
	s := newTestStore(t)

	// Create a group, monitor, and outage so we can test outage_id foreign key
	if err := s.CreateGroup(Group{ID: "g-test-1", Name: "Test Group", CreatedAt: time.Now()}); err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}
	if err := s.CreateMonitor(Monitor{ID: "mon-test-1", GroupID: "g-test-1", Name: "Test", URL: "https://example.com", Active: true, Interval: 60}); err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}
	if err := s.CreateOutage("mon-test-1", "down", "Service is down"); err != nil {
		t.Fatalf("CreateOutage failed: %v", err)
	}

	// Get the outage ID
	outages, _ := s.GetActiveOutages()
	if len(outages) == 0 {
		t.Fatal("No outages found")
	}
	outageID := outages[0].ID

	i := Incident{
		ID:             "inc-new-1",
		Title:          "Auto-detected Outage",
		Description:    "Service is down",
		Type:           "incident",
		Severity:       "critical",
		Status:         "investigating",
		StartTime:      time.Now(),
		CreatedAt:      time.Now(),
		Source:         "auto",
		OutageID:       &outageID,
		Public:         false,
		AffectedGroups: `["g-test-1"]`,
	}

	// Create with new fields
	if err := s.CreateIncident(i); err != nil {
		t.Fatalf("CreateIncident failed: %v", err)
	}

	// Read and verify new fields
	incident, err := s.GetIncidentByID("inc-new-1")
	if err != nil {
		t.Fatalf("GetIncidentByID failed: %v", err)
	}
	if incident == nil {
		t.Fatal("Incident not found")
	}
	if incident.Source != "auto" {
		t.Errorf("Source mismatch: got %s, want auto", incident.Source)
	}
	if incident.OutageID == nil || *incident.OutageID != outageID {
		t.Error("OutageID mismatch")
	}
	if incident.Public {
		t.Error("Public should be false")
	}

	// Cleanup
	_ = s.DeleteIncident("inc-new-1")
	_ = s.DeleteMonitor("mon-test-1")
	_ = s.DeleteGroup("g-test-1")
}

func TestSetIncidentPublic(t *testing.T) {
	s := newTestStore(t)

	i := Incident{
		ID:          "inc-public-1",
		Title:       "Test Incident",
		Description: "Test",
		Type:        "incident",
		Severity:    "minor",
		Status:      "investigating",
		StartTime:   time.Now(),
		Public:      false,
	}

	if err := s.CreateIncident(i); err != nil {
		t.Fatalf("CreateIncident failed: %v", err)
	}

	// Set public to true
	if err := s.SetIncidentPublic("inc-public-1", true); err != nil {
		t.Fatalf("SetIncidentPublic failed: %v", err)
	}

	// Verify
	incident, _ := s.GetIncidentByID("inc-public-1")
	if !incident.Public {
		t.Error("Public should be true")
	}

	// Set back to false
	if err := s.SetIncidentPublic("inc-public-1", false); err != nil {
		t.Fatalf("SetIncidentPublic failed: %v", err)
	}

	incident, _ = s.GetIncidentByID("inc-public-1")
	if incident.Public {
		t.Error("Public should be false")
	}

	// Cleanup
	_ = s.DeleteIncident("inc-public-1")
}

func TestIncidentUpdates(t *testing.T) {
	s := newTestStore(t)

	// Create incident first
	i := Incident{
		ID:          "inc-updates-1",
		Title:       "Test Incident",
		Description: "Test",
		Type:        "incident",
		Severity:    "major",
		Status:      "investigating",
		StartTime:   time.Now(),
	}

	if err := s.CreateIncident(i); err != nil {
		t.Fatalf("CreateIncident failed: %v", err)
	}

	// Create updates
	if err := s.CreateIncidentUpdate("inc-updates-1", "investigating", "Looking into the issue"); err != nil {
		t.Fatalf("CreateIncidentUpdate failed: %v", err)
	}
	if err := s.CreateIncidentUpdate("inc-updates-1", "identified", "Root cause found"); err != nil {
		t.Fatalf("CreateIncidentUpdate failed: %v", err)
	}
	if err := s.CreateIncidentUpdate("inc-updates-1", "resolved", "Fixed and deployed"); err != nil {
		t.Fatalf("CreateIncidentUpdate failed: %v", err)
	}

	// Get updates
	updates, err := s.GetIncidentUpdates("inc-updates-1")
	if err != nil {
		t.Fatalf("GetIncidentUpdates failed: %v", err)
	}

	if len(updates) != 3 {
		t.Fatalf("Expected 3 updates, got %d", len(updates))
	}

	// Verify order (chronological)
	if updates[0].Status != "investigating" {
		t.Errorf("First update should be investigating, got %s", updates[0].Status)
	}
	if updates[1].Status != "identified" {
		t.Errorf("Second update should be identified, got %s", updates[1].Status)
	}
	if updates[2].Status != "resolved" {
		t.Errorf("Third update should be resolved, got %s", updates[2].Status)
	}

	// Cleanup - deleting incident should cascade delete updates
	_ = s.DeleteIncident("inc-updates-1")
}

func TestGetPublicResolvedIncidents(t *testing.T) {
	s := newTestStore(t)

	now := time.Now()
	endTime := now.Add(time.Hour)

	// Create a public resolved incident
	public1 := Incident{
		ID:          "inc-pub-resolved-1",
		Title:       "Public Resolved",
		Description: "Test",
		Type:        "incident",
		Severity:    "major",
		Status:      "resolved",
		StartTime:   now,
		EndTime:     &endTime,
		Public:      true,
	}
	if err := s.CreateIncident(public1); err != nil {
		t.Fatalf("CreateIncident failed: %v", err)
	}

	// Create a private resolved incident
	private1 := Incident{
		ID:          "inc-priv-resolved-1",
		Title:       "Private Resolved",
		Description: "Test",
		Type:        "incident",
		Severity:    "minor",
		Status:      "resolved",
		StartTime:   now,
		EndTime:     &endTime,
		Public:      false,
	}
	if err := s.CreateIncident(private1); err != nil {
		t.Fatalf("CreateIncident failed: %v", err)
	}

	// Create a public but active incident
	publicActive := Incident{
		ID:          "inc-pub-active-1",
		Title:       "Public Active",
		Description: "Test",
		Type:        "incident",
		Severity:    "critical",
		Status:      "investigating",
		StartTime:   now,
		Public:      true,
	}
	if err := s.CreateIncident(publicActive); err != nil {
		t.Fatalf("CreateIncident failed: %v", err)
	}

	// Get public resolved incidents
	since := now.Add(-24 * time.Hour)
	incidents, err := s.GetPublicResolvedIncidents(since)
	if err != nil {
		t.Fatalf("GetPublicResolvedIncidents failed: %v", err)
	}

	// Should only return the public resolved one
	if len(incidents) != 1 {
		t.Fatalf("Expected 1 incident, got %d", len(incidents))
	}
	if incidents[0].ID != "inc-pub-resolved-1" {
		t.Errorf("Expected inc-pub-resolved-1, got %s", incidents[0].ID)
	}

	// Cleanup
	_ = s.DeleteIncident("inc-pub-resolved-1")
	_ = s.DeleteIncident("inc-priv-resolved-1")
	_ = s.DeleteIncident("inc-pub-active-1")
}

func TestGetPublicResolvedIncidentsExcludesMaintenance(t *testing.T) {
	s := newTestStore(t)

	now := time.Now()
	endTime := now.Add(time.Hour)

	// Create a public resolved incident (should be included)
	incident := Incident{
		ID:          "inc-resolved-1",
		Title:       "Resolved Incident",
		Description: "A real incident",
		Type:        "incident",
		Severity:    "major",
		Status:      "resolved",
		StartTime:   now,
		EndTime:     &endTime,
		Public:      true,
	}
	if err := s.CreateIncident(incident); err != nil {
		t.Fatalf("CreateIncident failed: %v", err)
	}

	// Create a public completed maintenance window (should be EXCLUDED)
	maintenance := Incident{
		ID:          "maint-completed-1",
		Title:       "Scheduled Maintenance",
		Description: "Planned downtime",
		Type:        "maintenance",
		Severity:    "minor",
		Status:      "completed",
		StartTime:   now,
		EndTime:     &endTime,
		Public:      true,
	}
	if err := s.CreateIncident(maintenance); err != nil {
		t.Fatalf("CreateIncident (maintenance) failed: %v", err)
	}

	// Get public resolved incidents
	since := now.Add(-24 * time.Hour)
	incidents, err := s.GetPublicResolvedIncidents(since)
	if err != nil {
		t.Fatalf("GetPublicResolvedIncidents failed: %v", err)
	}

	// Should only return the incident, NOT the maintenance window
	if len(incidents) != 1 {
		t.Fatalf("Expected 1 incident, got %d (maintenance windows should be excluded)", len(incidents))
	}
	if incidents[0].ID != "inc-resolved-1" {
		t.Errorf("Expected inc-resolved-1, got %s", incidents[0].ID)
	}
	if incidents[0].Type != "incident" {
		t.Errorf("Expected type 'incident', got '%s'", incidents[0].Type)
	}

	// Cleanup
	_ = s.DeleteIncident("inc-resolved-1")
	_ = s.DeleteIncident("maint-completed-1")
}
