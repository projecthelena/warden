package db

import (
	"testing"
	"time"
)

func TestSettingsResult(t *testing.T) {
	s := newTestStore(t)

	_, err := s.GetSetting("missing")
	if err == nil {
		t.Error("Expected error for missing setting")
	}

	if err := s.SetSetting("foo", "bar"); err != nil {
		t.Fatalf("SetSetting failed: %v", err)
	}

	val, err := s.GetSetting("foo")
	if err != nil {
		t.Fatalf("GetSetting failed: %v", err)
	}
	if val != "bar" {
		t.Errorf("Expected 'bar', got '%s'", val)
	}

	_ = s.SetSetting("foo", "baz")
	val, _ = s.GetSetting("foo")
	if val != "baz" {
		t.Errorf("Expected 'baz', got '%s'", val)
	}
}

func TestSystemStats(t *testing.T) {
	s := newTestStore(t)
	stats, err := s.GetSystemStats()
	if err != nil {
		t.Fatalf("GetSystemStats failed: %v", err)
	}

	// Should be empty mostly
	if stats.TotalMonitors != 0 {
		// If seed runs, it might not be 0 depending on seed logic.
		// Current seed creates groups but not monitors.
		t.Logf("Total monitors: %d", stats.TotalMonitors)
	}
}

// --- Digest queue tests ---

func insertTestDigestEvent(t *testing.T, s *Store, monitorID, eventType string, eventTime time.Time) {
	t.Helper()
	err := s.InsertDigestEvent(monitorID, "Monitor "+monitorID, "https://"+monitorID+".example.com", eventType, "test message", eventTime)
	if err != nil {
		t.Fatalf("InsertDigestEvent failed: %v", err)
	}
}

func TestDigestQueue_InsertAndGetUnsent(t *testing.T) {
	RunTestWithBothDBs(t, "InsertAndGetUnsent", func(t *testing.T, s *Store) {
		// Empty queue should return empty slice, not error.
		events, err := s.GetUnsentDigestEvents()
		if err != nil {
			t.Fatalf("GetUnsentDigestEvents on empty queue: %v", err)
		}
		if len(events) != 0 {
			t.Errorf("expected 0 events, got %d", len(events))
		}

		// Insert two events.
		t0 := time.Date(2026, 3, 25, 8, 0, 0, 0, time.UTC)
		insertTestDigestEvent(t, s, "m1", "down", t0)
		insertTestDigestEvent(t, s, "m2", "degraded", t0.Add(time.Hour))

		events, err = s.GetUnsentDigestEvents()
		if err != nil {
			t.Fatalf("GetUnsentDigestEvents: %v", err)
		}
		if len(events) != 2 {
			t.Fatalf("expected 2 events, got %d", len(events))
		}

		// Events should be ordered by event_time ASC.
		if events[0].MonitorID != "m1" {
			t.Errorf("expected first event from m1, got %s", events[0].MonitorID)
		}
		if events[1].MonitorID != "m2" {
			t.Errorf("expected second event from m2, got %s", events[1].MonitorID)
		}

		// Verify fields are populated correctly.
		if events[0].EventType != "down" {
			t.Errorf("expected event_type 'down', got %s", events[0].EventType)
		}
		if events[0].MonitorName != "Monitor m1" {
			t.Errorf("expected monitor name 'Monitor m1', got %s", events[0].MonitorName)
		}
	})
}

func TestDigestQueue_MarkSentRemovesFromUnsent(t *testing.T) {
	RunTestWithBothDBs(t, "MarkSentRemovesFromUnsent", func(t *testing.T, s *Store) {
		t0 := time.Date(2026, 3, 25, 8, 0, 0, 0, time.UTC)
		insertTestDigestEvent(t, s, "m1", "down", t0)
		insertTestDigestEvent(t, s, "m2", "degraded", t0.Add(time.Hour))

		events, err := s.GetUnsentDigestEvents()
		if err != nil {
			t.Fatalf("GetUnsentDigestEvents: %v", err)
		}
		if len(events) != 2 {
			t.Fatalf("expected 2 unsent events, got %d", len(events))
		}

		// Mark only the first event as sent.
		if err := s.MarkDigestEventsSent([]int64{events[0].ID}); err != nil {
			t.Fatalf("MarkDigestEventsSent: %v", err)
		}

		// Only one event should remain unsent.
		remaining, err := s.GetUnsentDigestEvents()
		if err != nil {
			t.Fatalf("GetUnsentDigestEvents after mark: %v", err)
		}
		if len(remaining) != 1 {
			t.Fatalf("expected 1 unsent event after marking, got %d", len(remaining))
		}
		if remaining[0].MonitorID != "m2" {
			t.Errorf("expected remaining event from m2, got %s", remaining[0].MonitorID)
		}

		// Mark the second event as sent.
		if err := s.MarkDigestEventsSent([]int64{remaining[0].ID}); err != nil {
			t.Fatalf("MarkDigestEventsSent second: %v", err)
		}

		// Queue should now be empty.
		final, err := s.GetUnsentDigestEvents()
		if err != nil {
			t.Fatalf("GetUnsentDigestEvents after all sent: %v", err)
		}
		if len(final) != 0 {
			t.Errorf("expected 0 unsent events after all marked, got %d", len(final))
		}
	})
}

func TestDigestQueue_MarkSentEmptySliceIsNoOp(t *testing.T) {
	RunTestWithBothDBs(t, "MarkSentEmptySlice", func(t *testing.T, s *Store) {
		// Should not error on empty ID slice.
		if err := s.MarkDigestEventsSent([]int64{}); err != nil {
			t.Errorf("MarkDigestEventsSent(empty) should be a no-op, got: %v", err)
		}
	})
}

func TestDigestQueue_MarkSentAllAtOnce(t *testing.T) {
	RunTestWithBothDBs(t, "MarkSentAllAtOnce", func(t *testing.T, s *Store) {
		t0 := time.Date(2026, 3, 25, 8, 0, 0, 0, time.UTC)
		for i := 0; i < 5; i++ {
			insertTestDigestEvent(t, s, "m1", "down", t0.Add(time.Duration(i)*time.Minute))
		}

		events, err := s.GetUnsentDigestEvents()
		if err != nil {
			t.Fatalf("GetUnsentDigestEvents: %v", err)
		}
		if len(events) != 5 {
			t.Fatalf("expected 5 events, got %d", len(events))
		}

		ids := make([]int64, len(events))
		for i, e := range events {
			ids[i] = e.ID
		}

		if err := s.MarkDigestEventsSent(ids); err != nil {
			t.Fatalf("MarkDigestEventsSent: %v", err)
		}

		remaining, err := s.GetUnsentDigestEvents()
		if err != nil {
			t.Fatalf("GetUnsentDigestEvents after bulk mark: %v", err)
		}
		if len(remaining) != 0 {
			t.Errorf("expected 0 remaining unsent, got %d", len(remaining))
		}
	})
}

func TestDigestQueue_PruneDeletesOldSentEvents(t *testing.T) {
	RunTestWithBothDBs(t, "PruneDeletesOldSent", func(t *testing.T, s *Store) {
		t0 := time.Date(2026, 3, 25, 8, 0, 0, 0, time.UTC)
		insertTestDigestEvent(t, s, "m1", "down", t0)

		events, err := s.GetUnsentDigestEvents()
		if err != nil {
			t.Fatalf("GetUnsentDigestEvents: %v", err)
		}
		if len(events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(events))
		}

		// Mark it as sent (sent_at = now in DB).
		if err := s.MarkDigestEventsSent([]int64{events[0].ID}); err != nil {
			t.Fatalf("MarkDigestEventsSent: %v", err)
		}

		// Pruning with 365 days should NOT delete it (just marked sent moments ago).
		if err := s.PruneDigestEvents(365); err != nil {
			t.Fatalf("PruneDigestEvents(365): %v", err)
		}

		// Manually set sent_at far in the past to simulate an old record.
		if s.IsPostgres() {
			_, _ = s.db.Exec("UPDATE notification_digest_queue SET sent_at = NOW() - INTERVAL '400 days'")
		} else {
			_, _ = s.db.Exec("UPDATE notification_digest_queue SET sent_at = datetime('now', '-400 days')")
		}

		// Pruning with 365 days should now delete the old record.
		if err := s.PruneDigestEvents(365); err != nil {
			t.Fatalf("PruneDigestEvents(365) after backdating: %v", err)
		}

		// Verify the record is gone.
		var count int
		_ = s.db.QueryRow("SELECT COUNT(*) FROM notification_digest_queue").Scan(&count)
		if count != 0 {
			t.Errorf("expected 0 rows after pruning old sent event, got %d", count)
		}
	})
}

func TestDigestQueue_PruneDoesNotDeleteUnsentEvents(t *testing.T) {
	RunTestWithBothDBs(t, "PruneDoesNotDeleteUnsent", func(t *testing.T, s *Store) {
		t0 := time.Date(2020, 1, 1, 8, 0, 0, 0, time.UTC) // very old event time
		insertTestDigestEvent(t, s, "m1", "down", t0)

		// Pruning should NOT delete unsent events regardless of event_time age,
		// because PruneDigestEvents only removes rows where sent = true.
		if err := s.PruneDigestEvents(1); err != nil {
			t.Fatalf("PruneDigestEvents: %v", err)
		}

		events, err := s.GetUnsentDigestEvents()
		if err != nil {
			t.Fatalf("GetUnsentDigestEvents: %v", err)
		}
		if len(events) != 1 {
			t.Errorf("expected unsent event to survive pruning, got %d events", len(events))
		}
	})
}

func TestDigestQueue_PruneInvalidDays(t *testing.T) {
	s := newTestStore(t)

	if err := s.PruneDigestEvents(0); err == nil {
		t.Error("PruneDigestEvents(0) should return error")
	}
	if err := s.PruneDigestEvents(-1); err == nil {
		t.Error("PruneDigestEvents(-1) should return error")
	}
	if err := s.PruneDigestEvents(3651); err == nil {
		t.Error("PruneDigestEvents(3651) should return error")
	}
	// Boundary values should succeed.
	if err := s.PruneDigestEvents(1); err != nil {
		t.Errorf("PruneDigestEvents(1) should succeed, got: %v", err)
	}
	if err := s.PruneDigestEvents(3650); err != nil {
		t.Errorf("PruneDigestEvents(3650) should succeed, got: %v", err)
	}
}
