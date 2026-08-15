package db

import (
	"fmt"
	"testing"
	"time"
)

// seedEvent inserts an event with an explicit timestamp so ordering is deterministic
// (CreateEvent stamps CURRENT_TIMESTAMP, and a tight loop would tie).
func seedEvent(t *testing.T, s *Store, monitorID, msg string, ts time.Time) {
	t.Helper()
	_, err := s.db.Exec(s.rebind(
		`INSERT INTO monitor_events (monitor_id, type, message, timestamp) VALUES (?, 'down', ?, ?)`),
		monitorID, msg, ts)
	if err != nil {
		t.Fatalf("seed event: %v", err)
	}
}

func TestGetRecentEventsForMonitors(t *testing.T) {
	RunTestWithBothDBs(t, "GetRecentEventsForMonitors", func(t *testing.T, s *Store) {
		_ = s.CreateGroup(Group{ID: "g1", Name: "G1"})
		_ = s.CreateMonitor(Monitor{ID: "m1", GroupID: "g1", Name: "M1", Interval: 60})
		_ = s.CreateMonitor(Monitor{ID: "m2", GroupID: "g1", Name: "M2", Interval: 60})

		base := time.Now().UTC().Add(-time.Hour)
		for i := 0; i < 15; i++ {
			seedEvent(t, s, "m1", fmt.Sprintf("m1-%d", i), base.Add(time.Duration(i)*time.Minute))
		}
		for i := 0; i < 3; i++ {
			seedEvent(t, s, "m2", fmt.Sprintf("m2-%d", i), base.Add(time.Duration(i)*time.Minute))
		}

		got, err := s.GetRecentEventsForMonitors([]string{"m1", "m2"}, 10)
		if err != nil {
			t.Fatalf("GetRecentEventsForMonitors: %v", err)
		}

		// Per-monitor cap: m1 clipped to 10, m2 keeps all 3.
		if len(got["m1"]) != 10 {
			t.Errorf("m1: got %d events, want 10", len(got["m1"]))
		}
		if len(got["m2"]) != 3 {
			t.Errorf("m2: got %d events, want 3", len(got["m2"]))
		}

		// Each bucket holds only that monitor's rows, newest first.
		for id, evs := range got {
			for i, e := range evs {
				if e.MonitorID != id {
					t.Errorf("%s bucket contained an event for %s", id, e.MonitorID)
				}
				if i > 0 && evs[i-1].Timestamp.Before(e.Timestamp) {
					t.Errorf("%s events not ordered newest-first", id)
				}
			}
		}

		// The batched result matches the per-monitor call it replaces, row for row.
		want, err := s.GetMonitorEvents("m1", 10)
		if err != nil {
			t.Fatalf("GetMonitorEvents: %v", err)
		}
		if len(want) != len(got["m1"]) {
			t.Fatalf("length mismatch: per-monitor %d, batched %d", len(want), len(got["m1"]))
		}
		for i := range want {
			if want[i].ID != got["m1"][i].ID {
				t.Errorf("event %d id mismatch: per-monitor %d, batched %d", i, want[i].ID, got["m1"][i].ID)
			}
		}
	})
}

func TestGetRecentEventsForMonitors_Edges(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreateGroup(Group{ID: "g1", Name: "G1"})
	_ = s.CreateMonitor(Monitor{ID: "m1", GroupID: "g1", Name: "M1", Interval: 60})

	// Empty ids -> empty map, no invalid "IN ()" SQL.
	m, err := s.GetRecentEventsForMonitors(nil, 10)
	if err != nil {
		t.Fatalf("empty ids: %v", err)
	}
	if len(m) != 0 {
		t.Errorf("expected empty map, got %d", len(m))
	}

	// A monitor with no events has no entry, not an error.
	m, err = s.GetRecentEventsForMonitors([]string{"m1"}, 10)
	if err != nil {
		t.Fatalf("no events: %v", err)
	}
	if len(m["m1"]) != 0 {
		t.Errorf("expected no events for m1, got %d", len(m["m1"]))
	}

	// Enriched detail survives the window query.
	_ = s.CreateEventWithDetails("m1", "down", "boom", &EventDetails{StatusCode: 503, ErrorMessage: "pool exhausted"})
	m, _ = s.GetRecentEventsForMonitors([]string{"m1"}, 10)
	if len(m["m1"]) != 1 {
		t.Fatalf("expected 1 event, got %d", len(m["m1"]))
	}
	e := m["m1"][0]
	if e.StatusCode == nil || *e.StatusCode != 503 || e.ErrorMessage == nil || *e.ErrorMessage != "pool exhausted" {
		t.Errorf("enriched detail lost: %+v", e)
	}
}
