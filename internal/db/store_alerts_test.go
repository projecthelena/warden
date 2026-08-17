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
