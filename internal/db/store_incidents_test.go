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
