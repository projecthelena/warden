package db

import (
	"reflect"
	"testing"
	"time"
)

func TestHTTPDiagnosticsPersistence(t *testing.T) {
	RunTestWithBothDBs(t, "diagnostics", func(t *testing.T, s *Store) {
		if err := s.CreateGroup(Group{ID: "g-trace", Name: "Trace"}); err != nil {
			t.Fatal(err)
		}
		if err := s.CreateMonitor(Monitor{ID: "m-trace", GroupID: "g-trace", Name: "Trace", URL: "https://example.test", Interval: 60}); err != nil {
			t.Fatal(err)
		}
		reused := false
		d := &HTTPDiagnostics{Attempts: []HTTPAttempt{{TotalMS: 42, Hops: []HTTPHop{{Host: "example.test", RemoteIP: "192.0.2.1", Reused: &reused, FailurePhase: "tls", Phases: []HTTPPhase{{Name: "tls", DurationMS: 40, Complete: true, Failed: true}}}}}}}
		if err := s.BatchInsertChecks([]CheckResult{{MonitorID: "m-trace", Timestamp: time.Now().UTC(), Status: "down", Diagnostics: d}, {MonitorID: "m-trace", Timestamp: time.Now().UTC().Add(-time.Minute), Status: "up"}}); err != nil {
			t.Fatal(err)
		}
		checks, err := s.GetMonitorChecks("m-trace", 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(checks) != 2 || !reflect.DeepEqual(checks[0].Diagnostics, d) || checks[1].Diagnostics != nil {
			t.Fatalf("checks: %+v", checks)
		}
		if err := s.CreateEventWithDetails("m-trace", "down", "Failed TLS", &EventDetails{Diagnostics: d}); err != nil {
			t.Fatal(err)
		}
		events, err := s.GetMonitorEvents("m-trace", 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(events) != 1 || !reflect.DeepEqual(events[0].Diagnostics, d) {
			t.Fatal("event trace lost")
		}
		between, err := s.GetMonitorEventsBetween("m-trace", time.Now().UTC().Add(-time.Hour), time.Now().UTC().Add(time.Hour), 10)
		if err != nil || len(between) != 1 || !reflect.DeepEqual(between[0].Diagnostics, d) {
			t.Fatalf("between: %v", err)
		}
		batch, err := s.GetRecentEventsForMonitors([]string{"m-trace"}, 10)
		if err != nil || !reflect.DeepEqual(batch["m-trace"][0].Diagnostics, d) {
			t.Fatalf("batch: %v", err)
		}
		global, err := s.GetEventsBetween(time.Now().UTC().Add(-time.Hour), time.Now().UTC().Add(time.Hour))
		if err != nil || len(global) != 1 || !reflect.DeepEqual(global[0].Diagnostics, d) {
			t.Fatalf("global: %v", err)
		}
	})
}

func TestCheckSummaryAndPagination(t *testing.T) {
	RunTestWithBothDBs(t, "check summary", func(t *testing.T, s *Store) {
		if err := s.CreateGroup(Group{ID: "g-summary", Name: "Summary"}); err != nil {
			t.Fatal(err)
		}
		if err := s.CreateMonitor(Monitor{ID: "m-summary", GroupID: "g-summary", Name: "Summary", URL: "https://example.test", Interval: 60}); err != nil {
			t.Fatal(err)
		}
		now := time.Now().UTC()
		d := &HTTPDiagnostics{Attempts: []HTTPAttempt{{Status: "down", FailurePhase: "tcp", Hops: []HTTPHop{}}, {Status: "up", Hops: []HTTPHop{}}}}
		if err := s.BatchInsertChecks([]CheckResult{{MonitorID: "m-summary", Timestamp: now, Status: "up", Diagnostics: d}, {MonitorID: "m-summary", Timestamp: now, Status: "down"}, {MonitorID: "m-summary", Timestamp: now.Add(-48 * time.Hour), Status: "down"}}); err != nil {
			t.Fatal(err)
		}
		summary, err := s.GetMonitorCheckSummary("m-summary", now.Add(-24*time.Hour), now.Add(time.Second))
		if err != nil || summary.Total != 2 || summary.Recovered != 1 || summary.Failed != 1 || summary.Traced != 1 || summary.FailurePhases["tcp"] != 1 {
			t.Fatalf("summary: %+v %v", summary, err)
		}
		page, err := s.GetMonitorChecksPage("m-summary", 2, 0)
		if err != nil || len(page) != 2 {
			t.Fatal(err)
		}
		if err := s.BatchInsertChecks([]CheckResult{{MonitorID: "m-summary", Timestamp: now, Status: "up"}}); err != nil {
			t.Fatal(err)
		}
		older, err := s.GetMonitorChecksPage("m-summary", 2, page[1].ID)
		if err != nil || len(older) != 1 || older[0].ID >= page[1].ID {
			t.Fatalf("unstable pagination: %+v %v", older, err)
		}
	})
}
