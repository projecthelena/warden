package db

import (
	"testing"
	"time"
)

func TestRecoveryQueuePreservesEpisodeUntilStable(t *testing.T) {
	RunTestWithBothDBs(t, "recovery episode", func(t *testing.T, s *Store) {
		seedOutageFixture(t, s)
		if _, err := s.db.Exec(s.rebind("UPDATE monitors SET active = ? WHERE id = ?"), true, "m1"); err != nil {
			t.Fatal(err)
		}
		if err := s.CreateOutage("m1", "down", "unavailable"); err != nil {
			t.Fatal(err)
		}
		outages, err := s.GetOpenOutages()
		if err != nil {
			t.Fatal(err)
		}
		now := time.Now().UTC()
		if _, err := s.MarkOutagesNotified([]int64{outages[0].ID}, now, "group-episode"); err != nil {
			t.Fatal(err)
		}
		recovered := now.Add(time.Minute)
		if err := s.CloseOutageQueueRecovery("m1", `{"Type":"up"}`, recovered, true); err != nil {
			t.Fatal(err)
		}
		closed, err := s.GetOutageByID(outages[0].ID)
		if err != nil {
			t.Fatal(err)
		}
		if closed.EndTime == nil || !closed.EndTime.Equal(recovered) {
			t.Fatalf("notification wait changed measured outage: %+v", closed)
		}
		pending, err := s.PendingNotificationRecoveries()
		if err != nil || len(pending) != 1 {
			t.Fatalf("pending: %v %v", pending, err)
		}
		if err := s.CreateOutageIfMonitorActive("m1", "down", "failed again"); err != nil {
			t.Fatal(err)
		}
		pending, err = s.PendingNotificationRecoveries()
		if err != nil || len(pending) != 0 {
			t.Fatalf("stale recovery remains: %v %v", pending, err)
		}
		outages, err = s.GetOpenOutages()
		if err != nil {
			t.Fatal(err)
		}
		if len(outages) != 1 || outages[0].NotifiedAt == nil || !outages[0].NotifiedAt.Equal(now) || outages[0].CorrelationID != "group-episode" {
			t.Fatalf("repeated failure lost original announcement: %+v", outages)
		}
		count, err := s.CountAnnouncedOutagesSince("m1", now.Add(-time.Hour))
		if err != nil || count != 1 {
			t.Fatalf("recurrence counted as a new interruption: %d %v", count, err)
		}
		if err := s.CloseOutageQueueRecovery("m1", `{"Type":"up"}`, recovered.Add(time.Minute), true); err != nil {
			t.Fatal(err)
		}
		claimed, err := s.ClaimNotificationRecovery("m1", recovered)
		if err != nil || claimed {
			t.Fatalf("old snapshot claimed new recovery: %v %v", claimed, err)
		}
		claimed, err = s.ClaimNotificationRecovery("m1", recovered.Add(time.Minute))
		if err != nil || !claimed {
			t.Fatalf("latest recovery not claimed: %v %v", claimed, err)
		}
		claimed, err = s.ClaimNotificationRecovery("m1", recovered.Add(time.Minute))
		if err != nil || claimed {
			t.Fatalf("duplicate recovery: %v %v", claimed, err)
		}
	})
}

func TestWorseningFromDegradedToDownStillAlerts(t *testing.T) {
	RunTestWithBothDBs(t, "recovery escalation", func(t *testing.T, s *Store) {
		seedOutageFixture(t, s)
		if _, err := s.db.Exec(s.rebind("UPDATE monitors SET active = ? WHERE id = ?"), true, "m1"); err != nil {
			t.Fatal(err)
		}
		if err := s.CreateOutage("m1", "degraded", "slow"); err != nil {
			t.Fatal(err)
		}
		outages, err := s.GetOpenOutages()
		if err != nil {
			t.Fatal(err)
		}
		now := time.Now().UTC()
		if _, err := s.MarkOutageNotified(outages[0].ID, now); err != nil {
			t.Fatal(err)
		}
		if err := s.CloseOutageQueueRecovery("m1", "{}", now.Add(time.Minute), true); err != nil {
			t.Fatal(err)
		}
		if err := s.CreateOutageIfMonitorActive("m1", "down", "unavailable"); err != nil {
			t.Fatal(err)
		}
		outages, err = s.GetOpenOutages()
		if err != nil {
			t.Fatal(err)
		}
		if len(outages) != 1 || outages[0].NotifiedAt != nil {
			t.Fatalf("escalation was silenced: %+v", outages)
		}
	})
}
