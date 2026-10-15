package db

import (
	"context"
	"github.com/projecthelena/warden/internal/metricstate"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func operationValue(t *testing.T, name, operation, outcome string) float64 {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.Metric {
			labels := map[string]string{}
			for _, label := range metric.Label {
				labels[label.GetName()] = label.GetValue()
			}
			if labels["operation"] != operation || labels["outcome"] != outcome {
				continue
			}
			if metric.Histogram != nil {
				return float64(metric.Histogram.GetSampleCount())
			}
			return metric.GetGauge().GetValue()
		}
	}
	return 0
}

func TestOperationMetricsBlockedConnectionAndFailure(t *testing.T) {
	RunTestWithBothDBs(t, "operation metrics", func(t *testing.T, s *Store) {
		seedBaselineMonitor(t, s)
		s.db.SetMaxOpenConns(1)
		conn, err := s.db.Conn(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = conn.Close() }()
		before := operationValue(t, "warden_db_operation_duration_seconds", "persist_connection", "success")
		done := make(chan error, 1)
		go func() {
			done <- s.BatchInsertChecks([]CheckResult{{MonitorID: "m1", Status: "up", Latency: 10, Timestamp: time.Now().UTC()}})
		}()
		deadline := time.Now().Add(3 * time.Second)
		for operationValue(t, "warden_db_operations_in_flight", "persist_connection", "") != 1 {
			if time.Now().After(deadline) {
				t.Fatal("blocked connection not exposed")
			}
			time.Sleep(time.Millisecond)
		}
		if got := operationValue(t, "warden_db_operation_duration_seconds", "persist_connection", "success"); got != before {
			t.Fatal("blocked operation counted as completed")
		}
		if err := conn.Close(); err != nil {
			t.Fatal(err)
		}
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("writer did not resume")
		}
		if got := operationValue(t, "warden_db_operations_in_flight", "persist_connection", ""); got != 0 {
			t.Fatalf("active gauge = %v", got)
		}
		if _, err := s.ComputeLatencyBaseline("m1", 24*time.Hour, 1, time.Now()); err != nil {
			t.Fatal(err)
		}
		for _, op := range []string{"baseline_count", "baseline_p50", "baseline_p95", "baseline_save", "persist_begin", "persist_insert", "persist_commit"} {
			if operationValue(t, "warden_db_operation_duration_seconds", op, "success") == 0 {
				t.Errorf("missing %s", op)
			}
		}
		if _, err := s.db.Exec("DROP TABLE monitor_checks"); err != nil {
			t.Fatal(err)
		}
		before = operationValue(t, "warden_db_operation_duration_seconds", "persist_insert", "error")
		commits := operationValue(t, "warden_db_operation_duration_seconds", "persist_commit", "success")
		if err := s.BatchInsertChecks([]CheckResult{checkAt("m1", "up", 1)}); err == nil {
			t.Fatal("expected insert error")
		}
		if got := operationValue(t, "warden_db_operation_duration_seconds", "persist_insert", "error"); got != before+1 {
			t.Fatalf("error count = %v", got)
		}
		if got := operationValue(t, "warden_db_operation_duration_seconds", "persist_commit", "success"); got != commits {
			t.Fatal("failed insert recorded commit")
		}
		if s.db.Stats().InUse != 0 {
			t.Fatal("connection leaked on error")
		}
		if operationValue(t, "warden_db_operations_in_flight", "persist_insert", "") != 0 {
			t.Fatal("failed insert still active")
		}
	})
}

func TestDisabledMetricsPreserveBatchWrites(t *testing.T) {
	metricstate.Configure(false)
	t.Cleanup(func() { metricstate.Configure(true) })
	RunTestWithBothDBs(t, "disabled metrics writes", func(t *testing.T, s *Store) {
		seedBaselineMonitor(t, s)
		before := operationValue(t, "warden_db_operation_duration_seconds", "persist_commit", "success")
		if err := s.BatchInsertChecks([]CheckResult{{MonitorID: "m1", Status: "up", Latency: 10, Timestamp: time.Now().UTC()}}); err != nil {
			t.Fatal(err)
		}
		rows, err := s.GetMonitorChecks("m1", 10)
		if err != nil || len(rows) != 1 {
			t.Fatalf("checks=%v err=%v", rows, err)
		}
		if operationValue(t, "warden_db_operation_duration_seconds", "persist_commit", "success") != before {
			t.Fatal("disabled write changed metrics")
		}
		if s.db.Stats().InUse != 0 {
			t.Fatal("connection leaked")
		}
	})
}
