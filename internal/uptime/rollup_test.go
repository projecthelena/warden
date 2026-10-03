package uptime

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/projecthelena/warden/internal/db"
	"github.com/prometheus/client_golang/prometheus"
)

func rollupMetric(t *testing.T, name, mode string) float64 {
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
			for _, label := range metric.Label {
				if label.GetName() == "mode" && label.GetValue() == mode {
					if metric.Gauge != nil {
						return metric.Gauge.GetValue()
					}
					return metric.Counter.GetValue()
				}
			}
		}
	}
	return 0
}

func TestRollupDiagnosticsFlag(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		name := "disabled"
		if enabled {
			name = "enabled"
		}
		t.Run(name, func(t *testing.T) {
			store, err := db.NewStore(db.NewTestConfig())
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = store.Close() }()
			m := NewManager(store)
			m.RollupDiagnostics = enabled
			if err := store.CreateMonitor(db.Monitor{ID: "rollup-test", Name: "rollup-test", GroupID: "g-default", Interval: 60}); err != nil {
				t.Fatal(err)
			}
			if err := store.BatchInsertChecks([]db.CheckResult{{MonitorID: "rollup-test", Status: "up", Timestamp: time.Now().UTC()}}); err != nil {
				t.Fatal(err)
			}
			before := rollupMetric(t, "warden_uptime_rollup_runs_total", name)
			if err := m.runRollup(name, 2); err != nil {
				t.Fatal(err)
			}
			got, err := store.GetDailyUptimeStatsForMonitors([]string{"rollup-test"}, 2)
			if err != nil {
				t.Fatal(err)
			}
			var checks int
			for _, day := range got["rollup-test"] {
				checks += day.Total
			}
			if checks != 1 {
				t.Fatalf("rollup must still run: got %d checks", checks)
			}
			want := 0.0
			if enabled {
				want = 1
			}
			if delta := rollupMetric(t, "warden_uptime_rollup_runs_total", name) - before; delta != want {
				t.Fatalf("diagnostic counter delta = %v, want %v", delta, want)
			}
			if got := rollupMetric(t, "warden_uptime_rollup_in_progress", name); got != 0 {
				t.Fatalf("in-progress gauge remains set: %v", got)
			}
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
			errorMode := name + "-error"
			before = rollupMetric(t, "warden_uptime_rollup_runs_total", errorMode)
			if err := m.runRollup(errorMode, 2); err == nil {
				t.Fatal("closed database must fail")
			}
			if delta := rollupMetric(t, "warden_uptime_rollup_runs_total", errorMode) - before; delta != want {
				t.Fatalf("failed-run counter delta = %v, want %v", delta, want)
			}
			if got := rollupMetric(t, "warden_uptime_rollup_in_progress", errorMode); got != 0 {
				t.Fatalf("failed run left in-progress gauge set: %v", got)
			}
		})
	}
}

func TestLogSlowRollup(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	stats := db.DailyUptimeRollupStats{Rows: 4, QueryDuration: time.Second}
	logSlowRollup("refresh", "sqlite", stats, slowRollupThreshold-time.Nanosecond, nil)
	if output.Len() != 0 {
		t.Fatal("fast rollup emitted a diagnostic log")
	}
	logSlowRollup("refresh", "sqlite", stats, slowRollupThreshold, errors.New("secret-driver-error"))
	for _, field := range []string{`"outcome":"error"`, `"pool_before"`, `"pool_after"`, `"query"`, `"read_connection"`} {
		if !strings.Contains(output.String(), field) {
			t.Errorf("missing structured field %s", field)
		}
	}
	if strings.Contains(output.String(), "secret-driver-error") {
		t.Fatal("driver error leaked into diagnostic log")
	}
}
