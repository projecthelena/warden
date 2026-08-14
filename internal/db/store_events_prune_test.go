package db

import (
	"testing"
	"time"
)

func seedPruneMonitor(t *testing.T, s *Store) {
	t.Helper()
	if err := s.CreateGroup(Group{ID: "g1", Name: "G1"}); err != nil {
		t.Fatalf("failed to seed group: %v", err)
	}
	if err := s.CreateMonitor(Monitor{
		ID: "m1", GroupID: "g1", Name: "Prune", URL: "https://example.com", Active: true, Interval: 60,
	}); err != nil {
		t.Fatalf("failed to seed monitor: %v", err)
	}
}

// Events were the one table retention never covered, and they are the heaviest: each
// failed check can carry a couple of kilobytes of captured response body.
func TestMultiDB_PruneMonitorEvents(t *testing.T) {
	RunTestWithBothDBs(t, "PruneMonitorEvents", func(t *testing.T, s *Store) {
		seedPruneMonitor(t, s)

		if err := s.CreateEvent("m1", "down", "recent"); err != nil {
			t.Fatalf("failed to seed event: %v", err)
		}
		old := time.Now().AddDate(0, 0, -40).UTC()
		if _, err := s.db.Exec(s.rebind("INSERT INTO monitor_events (monitor_id, type, message, timestamp) VALUES (?, ?, ?, ?)"),
			"m1", "down", "old", old); err != nil {
			t.Fatalf("failed to seed the old event: %v", err)
		}

		if err := s.PruneMonitorEvents(30); err != nil {
			t.Fatalf("prune failed: %v", err)
		}

		events, err := s.GetMonitorEvents("m1", 10)
		if err != nil {
			t.Fatalf("failed to read events: %v", err)
		}
		if len(events) != 1 || events[0].Message != "recent" {
			t.Fatalf("expected only the recent event to survive, got %d: %+v", len(events), events)
		}
	})
}

// Incidents are what people look at months later, and they live in their own table, so
// they have to outlive the events behind them.
func TestPruneMonitorEventsLeavesOutagesAlone(t *testing.T) {
	s := newTestStore(t)
	seedPruneMonitor(t, s)

	if err := s.CreateOutage("m1", "down", "Monitor is down"); err != nil {
		t.Fatalf("failed to seed outage: %v", err)
	}
	if err := s.CreateEvent("m1", "down", "Monitor is down"); err != nil {
		t.Fatalf("failed to seed event: %v", err)
	}

	if err := s.PruneMonitorEvents(1); err != nil {
		t.Fatalf("prune failed: %v", err)
	}

	outages, err := s.GetActiveOutages()
	if err != nil {
		t.Fatalf("failed to read outages: %v", err)
	}
	if len(outages) != 1 {
		t.Errorf("expected the outage to survive, got %d", len(outages))
	}
}

func TestPruneMonitorEventsRejectsNonsenseWindows(t *testing.T) {
	s := newTestStore(t)

	for _, days := range []int{0, -1, 3651} {
		if err := s.PruneMonitorEvents(days); err == nil {
			t.Errorf("expected %d days to be refused", days)
		}
	}
}
