package db

import (
	"testing"
	"time"
)

func TestGroupCRUD(t *testing.T) {
	s := newTestStore(t)

	// Create
	g := Group{ID: "g1", Name: "Group 1", CreatedAt: time.Now()}
	if err := s.CreateGroup(g); err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}

	// Read (indirectly via GetGroups)
	groups, err := s.GetGroups()
	if err != nil {
		t.Fatalf("GetGroups failed: %v", err)
	}
	found := false
	for _, grp := range groups {
		if grp.ID == "g1" {
			found = true
			if grp.Name != "Group 1" {
				t.Errorf("Name mismatch")
			}
		}
	}
	if !found {
		t.Error("Group g1 not found")
	}

	// Update
	if err := s.UpdateGroup("g1", "Updated Group 1"); err != nil {
		t.Fatalf("UpdateGroup failed: %v", err)
	}

	// Verify Update
	groups, _ = s.GetGroups()
	for _, grp := range groups {
		if grp.ID == "g1" && grp.Name != "Updated Group 1" {
			t.Errorf("Group update failed")
		}
	}

	// Delete
	if err := s.DeleteGroup("g1"); err != nil {
		t.Fatalf("DeleteGroup failed: %v", err)
	}

	// Verify Delete
	groups, _ = s.GetGroups()
	// Note: s.seed() creates default group, so count might contain 'g-default'.
	// Just check g1 is gone.
	for _, grp := range groups {
		if grp.ID == "g1" {
			t.Error("Group g1 should be deleted")
		}
	}
}

func TestGetGroupsWithMonitors(t *testing.T) {
	s := newTestStore(t)
	// Clear potential seed data for clean count test, or just explicitly check IDs
	// Clear potential seed data for clean count test, or just explicitly check IDs
	_, _ = s.db.Exec("DELETE FROM groups")
	_, _ = s.db.Exec("DELETE FROM monitors")

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
