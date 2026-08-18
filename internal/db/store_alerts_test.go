package db

import (
	"testing"
	"time"
)

func seedOutageFixture(t *testing.T, s *Store) {
	t.Helper()
	if err := s.CreateGroup(Group{ID: "g1", Name: "G1"}); err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if err := s.CreateMonitor(Monitor{ID: "m1", GroupID: "g1", Name: "Jellyfin", URL: "https://jellyfin.example.com", Interval: 60}); err != nil {
		t.Fatalf("CreateMonitor: %v", err)
	}
}

func TestGetOpenOutages_OnlyOpenWithMonitorDetail(t *testing.T) {
	RunTestWithBothDBs(t, "open outages", func(t *testing.T, s *Store) {
		seedOutageFixture(t, s)

		if err := s.CreateOutage("m1", "down", "Monitor is down (Status: 503)"); err != nil {
			t.Fatalf("CreateOutage: %v", err)
		}

		open, err := s.GetOpenOutages()
		if err != nil {
			t.Fatalf("GetOpenOutages: %v", err)
		}
		if len(open) != 1 {
			t.Fatalf("expected 1 open outage, got %d", len(open))
		}
		o := open[0]
		if o.MonitorName != "Jellyfin" || o.MonitorURL != "https://jellyfin.example.com" || o.GroupID != "g1" {
			t.Errorf("outage did not carry monitor detail: %+v", o)
		}
		if o.Summary != "Monitor is down (Status: 503)" {
			t.Errorf("summary = %q", o.Summary)
		}
		if o.NotifiedAt != nil || o.LastReminderAt != nil {
			t.Errorf("a fresh outage must start un-announced, got notified=%v reminded=%v", o.NotifiedAt, o.LastReminderAt)
		}

		// Closing it takes it out of the evaluator's view.
		if err := s.CloseOutage("m1"); err != nil {
			t.Fatalf("CloseOutage: %v", err)
		}
		open, err = s.GetOpenOutages()
		if err != nil {
			t.Fatalf("GetOpenOutages after close: %v", err)
		}
		if len(open) != 0 {
			t.Errorf("expected no open outages after close, got %d", len(open))
		}
	})
}

// Two evaluator ticks can overlap on a slow database. The claim has to be exclusive or the
// same outage alerts twice, which is precisely the noise this whole layer exists to stop.
func TestMarkOutageNotified_ClaimsOnlyOnce(t *testing.T) {
	RunTestWithBothDBs(t, "notify claim", func(t *testing.T, s *Store) {
		seedOutageFixture(t, s)
		if err := s.CreateOutage("m1", "down", "Monitor is down"); err != nil {
			t.Fatalf("CreateOutage: %v", err)
		}
		open, _ := s.GetOpenOutages()
		id := open[0].ID
		now := time.Now().UTC()

		claimed, err := s.MarkOutageNotified(id, now)
		if err != nil {
			t.Fatalf("MarkOutageNotified: %v", err)
		}
		if !claimed {
			t.Fatal("first claim should win")
		}

		claimed, err = s.MarkOutageNotified(id, now.Add(time.Minute))
		if err != nil {
			t.Fatalf("second MarkOutageNotified: %v", err)
		}
		if claimed {
			t.Error("second claim on an already-announced outage should lose")
		}

		open, _ = s.GetOpenOutages()
		if open[0].NotifiedAt == nil {
			t.Fatal("notified_at was not persisted")
		}
	})
}

func TestMarkOutageReminded_Persists(t *testing.T) {
	RunTestWithBothDBs(t, "reminder stamp", func(t *testing.T, s *Store) {
		seedOutageFixture(t, s)
		if err := s.CreateOutage("m1", "down", "Monitor is down"); err != nil {
			t.Fatalf("CreateOutage: %v", err)
		}
		open, _ := s.GetOpenOutages()
		at := time.Now().UTC().Truncate(time.Second)

		if err := s.MarkOutageReminded(open[0].ID, at); err != nil {
			t.Fatalf("MarkOutageReminded: %v", err)
		}

		open, _ = s.GetOpenOutages()
		if open[0].LastReminderAt == nil {
			t.Fatal("last_reminder_at was not persisted")
		}
		if got := open[0].LastReminderAt.UTC().Truncate(time.Second); !got.Equal(at) {
			t.Errorf("last_reminder_at = %v, want %v", got, at)
		}
	})
}

// The recovery message hangs off this answer: an outage nobody was told about must not
// produce a "recovered".
func TestCloseOutageReport_ReportsWhetherItWasAnnounced(t *testing.T) {
	RunTestWithBothDBs(t, "close report", func(t *testing.T, s *Store) {
		seedOutageFixture(t, s)

		// Never announced — a blip that resolved inside the silent window.
		if err := s.CreateOutage("m1", "down", "Monitor is down"); err != nil {
			t.Fatalf("CreateOutage: %v", err)
		}
		alerted, err := s.CloseOutageReport("m1")
		if err != nil {
			t.Fatalf("CloseOutageReport: %v", err)
		}
		if alerted {
			t.Error("a silent outage must not report as announced")
		}

		// Announced — the recovery is worth a message.
		if err := s.CreateOutage("m1", "down", "Monitor is down"); err != nil {
			t.Fatalf("CreateOutage: %v", err)
		}
		open, _ := s.GetOpenOutages()
		if _, err := s.MarkOutageNotified(open[0].ID, time.Now().UTC()); err != nil {
			t.Fatalf("MarkOutageNotified: %v", err)
		}
		alerted, err = s.CloseOutageReport("m1")
		if err != nil {
			t.Fatalf("CloseOutageReport: %v", err)
		}
		if !alerted {
			t.Error("an announced outage must report as announced")
		}

		// And it really is closed.
		open, _ = s.GetOpenOutages()
		if len(open) != 0 {
			t.Errorf("expected no open outages, got %d", len(open))
		}
	})
}

func TestCloseOutageReport_NoOpenOutage(t *testing.T) {
	RunTestWithBothDBs(t, "close report without outage", func(t *testing.T, s *Store) {
		seedOutageFixture(t, s)
		alerted, err := s.CloseOutageReport("m1")
		if err != nil {
			t.Fatalf("CloseOutageReport with nothing open: %v", err)
		}
		if alerted {
			t.Error("nothing open means nothing to announce")
		}
	})
}

// The evaluator works from a snapshot, so a monitor can recover between the read and the
// stamp. Stamping a closed outage announces a monitor that is already back up, and no
// "recovered" ever follows because the recovery path read notified_at before the stamp
// landed.
func TestMarkOutageNotified_RefusesAClosedOutage(t *testing.T) {
	RunTestWithBothDBs(t, "closed outage guard", func(t *testing.T, s *Store) {
		seedOutageFixture(t, s)
		if err := s.CreateOutage("m1", "down", "Monitor is down"); err != nil {
			t.Fatalf("CreateOutage: %v", err)
		}
		open, _ := s.GetOpenOutages()
		id := open[0].ID

		// The monitor recovers first — this is the CloseOutageReport the result processor
		// runs, which decides no recovery is worth announcing.
		alerted, err := s.CloseOutageReport("m1")
		if err != nil {
			t.Fatalf("CloseOutageReport: %v", err)
		}
		if alerted {
			t.Fatal("a silent outage must not report as announced")
		}

		// Now the evaluator, holding its stale snapshot, tries to announce it.
		claimed, err := s.MarkOutageNotified(id, time.Now().UTC())
		if err != nil {
			t.Fatalf("MarkOutageNotified: %v", err)
		}
		if claimed {
			t.Error("a closed outage was claimed for an alert — that alert would never get a recovery")
		}
	})
}

func TestMarkOutageReminded_RefusesAClosedOutage(t *testing.T) {
	RunTestWithBothDBs(t, "closed reminder guard", func(t *testing.T, s *Store) {
		seedOutageFixture(t, s)
		if err := s.CreateOutage("m1", "down", "Monitor is down"); err != nil {
			t.Fatalf("CreateOutage: %v", err)
		}
		open, _ := s.GetOpenOutages()
		id := open[0].ID
		if _, err := s.MarkOutageNotified(id, time.Now().UTC()); err != nil {
			t.Fatalf("MarkOutageNotified: %v", err)
		}
		if err := s.CloseOutage("m1"); err != nil {
			t.Fatalf("CloseOutage: %v", err)
		}

		if err := s.MarkOutageReminded(id, time.Now().UTC()); err != nil {
			t.Fatalf("MarkOutageReminded: %v", err)
		}

		var reminded interface{}
		if err := s.db.QueryRow(s.rebind("SELECT last_reminder_at FROM monitor_outages WHERE id = ?"), id).Scan(&reminded); err != nil {
			t.Fatalf("readback: %v", err)
		}
		if reminded != nil {
			t.Error("a closed outage was stamped as reminded")
		}
	})
}

// start_time is load-bearing now: the sustained ladder measures against it. Leaving it to
// CURRENT_TIMESTAMP means SQLite writes UTC and Postgres writes the session's local time,
// so on a non-UTC server the elapsed time would be wrong by the offset.
func TestCreateOutage_StampsStartTimeInUTC(t *testing.T) {
	RunTestWithBothDBs(t, "outage clock", func(t *testing.T, s *Store) {
		seedOutageFixture(t, s)

		before := time.Now().UTC().Add(-2 * time.Second)
		if err := s.CreateOutage("m1", "down", "Monitor is down"); err != nil {
			t.Fatalf("CreateOutage: %v", err)
		}
		after := time.Now().UTC().Add(2 * time.Second)

		open, err := s.GetOpenOutages()
		if err != nil || len(open) != 1 {
			t.Fatalf("GetOpenOutages: %v", err)
		}
		got := open[0].StartTime.UTC()
		if got.Before(before) || got.After(after) {
			t.Errorf("start_time = %v, expected it to sit between %v and %v — the clocks disagree", got, before, after)
		}

		// And the elapsed time the alerting ladder computes has to be non-negative.
		if elapsed := time.Now().UTC().Sub(got); elapsed < 0 {
			t.Errorf("elapsed since the outage opened is negative (%v): alerts would never fire", elapsed)
		}
	})
}
