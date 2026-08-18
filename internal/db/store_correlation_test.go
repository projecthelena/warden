package db

import (
	"testing"
	"time"
)

func seedCorrelationFixture(t *testing.T, s *Store, ids ...string) {
	t.Helper()
	if err := s.CreateGroup(Group{ID: "g1", Name: "NodeSource"}); err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	for _, id := range ids {
		if err := s.CreateMonitor(Monitor{
			ID: id, GroupID: "g1", Name: "monitor-" + id,
			URL: "https://" + id + ".example.com", Interval: 60,
		}); err != nil {
			t.Fatalf("CreateMonitor(%s): %v", id, err)
		}
	}
}

func openOutageIDs(t *testing.T, s *Store) []int64 {
	t.Helper()
	open, err := s.GetOpenOutages()
	if err != nil {
		t.Fatalf("GetOpenOutages: %v", err)
	}
	return outageIDsFrom(open)
}

func outageIDsFrom(open []OpenOutage) []int64 {
	ids := make([]int64, 0, len(open))
	for _, o := range open {
		ids = append(ids, o.ID)
	}
	return ids
}

// Every member of a correlated incident has to end up stamped under the same id, or the
// ones left behind fire their own alerts on the next tick.
func TestMarkOutagesNotified_StampsTheWholeSet(t *testing.T) {
	RunTestWithBothDBs(t, "batch stamp", func(t *testing.T, s *Store) {
		seedCorrelationFixture(t, s, "m1", "m2", "m3")
		for _, id := range []string{"m1", "m2", "m3"} {
			if err := s.CreateOutage(id, "down", "Monitor is down (Status: 404)"); err != nil {
				t.Fatalf("CreateOutage: %v", err)
			}
		}

		now := time.Now().UTC()
		claimed, err := s.MarkOutagesNotified(openOutageIDs(t, s), now, "group-1-123")
		if err != nil {
			t.Fatalf("MarkOutagesNotified: %v", err)
		}
		if claimed != 3 {
			t.Fatalf("claimed %d, want 3", claimed)
		}

		open, _ := s.GetOpenOutages()
		for _, o := range open {
			if o.NotifiedAt == nil {
				t.Errorf("%s left un-stamped", o.MonitorID)
			}
			if o.CorrelationID != "group-1-123" {
				t.Errorf("%s has correlation id %q, want them all to share one", o.MonitorID, o.CorrelationID)
			}
		}
	})
}

// A member already announced on its own keeps its original stamp and is not counted again.
// The count is what tells the evaluator whether it has anything new to say.
func TestMarkOutagesNotified_SkipsWhatWasAlreadyAnnounced(t *testing.T) {
	RunTestWithBothDBs(t, "partial claim", func(t *testing.T, s *Store) {
		seedCorrelationFixture(t, s, "m1", "m2")
		for _, id := range []string{"m1", "m2"} {
			if err := s.CreateOutage(id, "down", "Monitor is down"); err != nil {
				t.Fatalf("CreateOutage: %v", err)
			}
		}

		open, _ := s.GetOpenOutages()
		earlier := time.Now().UTC().Add(-time.Hour)
		if _, err := s.MarkOutageNotified(open[0].ID, earlier); err != nil {
			t.Fatalf("MarkOutageNotified: %v", err)
		}

		now := time.Now().UTC()
		claimed, err := s.MarkOutagesNotified(outageIDsFrom(open), now, "group-2")
		if err != nil {
			t.Fatalf("MarkOutagesNotified: %v", err)
		}
		if claimed != 1 {
			t.Fatalf("claimed %d, want 1 (the other was already announced)", claimed)
		}

		after, _ := s.GetOpenOutages()
		for _, o := range after {
			if o.ID == open[0].ID {
				if o.CorrelationID != "" {
					t.Errorf("an already-announced outage was pulled into the incident: %q", o.CorrelationID)
				}
				if !o.NotifiedAt.UTC().Truncate(time.Second).Equal(earlier.Truncate(time.Second)) {
					t.Errorf("its original stamp was overwritten: %v", o.NotifiedAt)
				}
			}
		}
	})
}

// A closed outage must not be stamped, for the same reason a single one must not: the
// alert would name a monitor that is already back up and no recovery would follow.
func TestMarkOutagesNotified_RefusesClosedOutages(t *testing.T) {
	RunTestWithBothDBs(t, "batch closed guard", func(t *testing.T, s *Store) {
		seedCorrelationFixture(t, s, "m1", "m2")
		for _, id := range []string{"m1", "m2"} {
			if err := s.CreateOutage(id, "down", "Monitor is down"); err != nil {
				t.Fatalf("CreateOutage: %v", err)
			}
		}
		ids := openOutageIDs(t, s)

		if err := s.CloseOutage("m1"); err != nil {
			t.Fatalf("CloseOutage: %v", err)
		}

		claimed, err := s.MarkOutagesNotified(ids, time.Now().UTC(), "group-3")
		if err != nil {
			t.Fatalf("MarkOutagesNotified: %v", err)
		}
		if claimed != 1 {
			t.Errorf("claimed %d, want 1 — the recovered monitor should have been refused", claimed)
		}
	})
}

func TestMarkOutagesNotified_EmptySet(t *testing.T) {
	RunTestWithBothDBs(t, "empty batch", func(t *testing.T, s *Store) {
		seedCorrelationFixture(t, s, "m1")
		claimed, err := s.MarkOutagesNotified(nil, time.Now().UTC(), "group-4")
		if err != nil {
			t.Fatalf("MarkOutagesNotified(nil): %v", err)
		}
		if claimed != 0 {
			t.Errorf("claimed %d from an empty set", claimed)
		}
	})
}

// The input to the repeat-offender damping. Only announced outages count, and only those
// inside the window, or a monitor that misbehaved last week would stay muted.
func TestCountAnnouncedOutagesSince(t *testing.T) {
	RunTestWithBothDBs(t, "announced count", func(t *testing.T, s *Store) {
		seedCorrelationFixture(t, s, "m1", "m2")
		now := time.Now().UTC()

		mk := func(monitorID string, notifiedAt *time.Time) {
			if err := s.CreateOutage(monitorID, "down", "Monitor is down"); err != nil {
				t.Fatalf("CreateOutage: %v", err)
			}
			open, _ := s.GetOpenOutages()
			var id int64
			for _, o := range open {
				if o.MonitorID == monitorID && o.NotifiedAt == nil {
					id = o.ID
				}
			}
			if notifiedAt != nil {
				if _, err := s.MarkOutageNotified(id, *notifiedAt); err != nil {
					t.Fatalf("MarkOutageNotified: %v", err)
				}
			}
			if err := s.CloseOutage(monitorID); err != nil {
				t.Fatalf("CloseOutage: %v", err)
			}
		}

		recent := now.Add(-time.Hour)
		old := now.AddDate(0, 0, -2)
		mk("m1", &recent)
		mk("m1", &recent)
		mk("m1", &old)    // outside a 24h window
		mk("m1", nil)     // never announced
		mk("m2", &recent) // a different monitor

		got, err := s.CountAnnouncedOutagesSince("m1", now.Add(-24*time.Hour))
		if err != nil {
			t.Fatalf("CountAnnouncedOutagesSince: %v", err)
		}
		if got != 2 {
			t.Errorf("count = %d, want 2 (announced, this monitor, inside the window)", got)
		}

		// Widening the window brings the older episode back.
		got, _ = s.CountAnnouncedOutagesSince("m1", now.AddDate(0, 0, -7))
		if got != 3 {
			t.Errorf("count over a week = %d, want 3", got)
		}

		got, _ = s.CountAnnouncedOutagesSince("m-unknown", now.Add(-24*time.Hour))
		if got != 0 {
			t.Errorf("count for an unknown monitor = %d, want 0", got)
		}
	})
}

// The flag has to round-trip through both the monitor list and the outage query, because
// the evaluator reads it from the second one.
func TestSetMonitorAlertsMuted(t *testing.T) {
	RunTestWithBothDBs(t, "mute flag", func(t *testing.T, s *Store) {
		seedCorrelationFixture(t, s, "m1")

		monitors, err := s.GetMonitors()
		if err != nil {
			t.Fatalf("GetMonitors: %v", err)
		}
		if monitors[0].AlertsMuted {
			t.Error("a new monitor should start audible")
		}

		if err := s.SetMonitorAlertsMuted("m1", true); err != nil {
			t.Fatalf("SetMonitorAlertsMuted: %v", err)
		}
		monitors, _ = s.GetMonitors()
		if !monitors[0].AlertsMuted {
			t.Fatal("the mute did not persist")
		}

		if err := s.CreateOutage("m1", "down", "Monitor is down"); err != nil {
			t.Fatalf("CreateOutage: %v", err)
		}
		open, _ := s.GetOpenOutages()
		if !open[0].AlertsMuted {
			t.Error("the outage query did not carry the mute, so the evaluator cannot honour it")
		}

		if err := s.SetMonitorAlertsMuted("m1", false); err != nil {
			t.Fatalf("SetMonitorAlertsMuted(false): %v", err)
		}
		open, _ = s.GetOpenOutages()
		if open[0].AlertsMuted {
			t.Error("unmuting did not take effect")
		}
	})
}

// The group name is what the correlated message is written around, so it has to come back
// with the outage rather than being looked up separately.
func TestGetOpenOutages_CarriesTheGroupName(t *testing.T) {
	RunTestWithBothDBs(t, "group name", func(t *testing.T, s *Store) {
		seedCorrelationFixture(t, s, "m1")
		if err := s.CreateOutage("m1", "down", "Monitor is down"); err != nil {
			t.Fatalf("CreateOutage: %v", err)
		}

		open, _ := s.GetOpenOutages()
		if open[0].GroupName != "NodeSource" {
			t.Errorf("GroupName = %q, want NodeSource", open[0].GroupName)
		}
		if open[0].CorrelationID != "" {
			t.Errorf("a fresh outage should have no correlation id, got %q", open[0].CorrelationID)
		}
	})
}
