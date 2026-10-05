package db

import (
	"testing"
	"time"
)

func TestInsightCheckSelectionAndOwnership(t *testing.T) {
	RunTestWithBothDBs(t, "insight check evidence", func(t *testing.T, s *Store) {
		if err := s.CreateGroup(Group{ID: "g-default", Name: "Evidence"}); err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{"m-evidence", "m-other"} {
			if err := s.CreateMonitor(Monitor{ID: id, GroupID: "g-default", Name: id, URL: "https://example.test", Interval: 60}); err != nil {
				t.Fatal(err)
			}
		}
		start := time.Now().UTC().Truncate(time.Hour).Add(-48 * time.Hour)
		rows := []CheckResult{
			{MonitorID: "m-evidence", Timestamp: start, Status: "up", Latency: 200},
			{MonitorID: "m-evidence", Timestamp: start.Add(time.Minute), Status: "up", Latency: 400, Diagnostics: &HTTPDiagnostics{Attempts: []HTTPAttempt{{Status: "up"}}}},
			{MonitorID: "m-evidence", Timestamp: start.Add(2 * time.Minute), Status: "down", Latency: 400},
			{MonitorID: "m-other", Timestamp: start.Add(3 * time.Minute), Status: "up", Latency: 350},
		}
		for i := 0; i < 30; i++ {
			rows = append(rows, CheckResult{MonitorID: "m-evidence", Timestamp: start.Add(24*time.Hour + time.Duration(i)*time.Minute), Status: "up", Latency: 10})
		}
		if err := s.BatchInsertChecks(rows); err != nil {
			t.Fatal(err)
		}
		ref, err := s.FindInsightCheck("m-evidence", start, start.Add(time.Hour), "up", 350)
		if err != nil || ref == nil || !ref.HasTrace || !ref.Timestamp.Equal(start.Add(time.Minute)) {
			t.Fatalf("wrong matching reference: %+v %v", ref, err)
		}
		exact, err := s.GetMonitorCheck("m-evidence", ref.ID)
		if err != nil || len(exact) != 1 || exact[0].Latency != 400 || exact[0].Diagnostics == nil {
			t.Fatalf("old check inaccessible: %+v %v", exact, err)
		}
		foreign, err := s.GetMonitorCheck("m-other", ref.ID)
		if err != nil || len(foreign) != 0 {
			t.Fatalf("cross-monitor evidence leak: %+v %v", foreign, err)
		}
		failed, err := s.FindInsightCheck("m-evidence", start, start.Add(time.Hour), "down", -1)
		if err != nil || failed == nil || failed.HasTrace || !failed.Timestamp.Equal(start.Add(2*time.Minute)) {
			t.Fatalf("wrong untraced failure: %+v %v", failed, err)
		}
		missing, err := s.FindInsightCheck("m-evidence", start.Add(time.Hour), start.Add(2*time.Hour), "down", -1)
		if err != nil || missing != nil {
			t.Fatalf("substituted an unrelated check: %+v %v", missing, err)
		}
		if _, err := s.db.Exec(s.rebind("DELETE FROM monitor_checks WHERE id = ?"), ref.ID); err != nil {
			t.Fatal(err)
		}
		expired, err := s.GetMonitorCheck("m-evidence", ref.ID)
		if err != nil || len(expired) != 0 {
			t.Fatalf("expired evidence substituted: %+v %v", expired, err)
		}
	})
}
