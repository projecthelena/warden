package uptime

import (
	"testing"
	"time"

	"github.com/projecthelena/warden/internal/db"
	"github.com/prometheus/client_golang/prometheus"
)

func baselineProcessed(t *testing.T, outcome string) float64 {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		if family.GetName() != "warden_baseline_monitors_total" {
			continue
		}
		for _, metric := range family.Metric {
			for _, label := range metric.Label {
				if label.GetName() == "outcome" && label.GetValue() == outcome {
					return metric.GetCounter().GetValue()
				}
			}
		}
	}
	return 0
}

func TestBaselineMetricsOutcomes(t *testing.T) {
	m, store := newAlertTestManager(t)
	m.baselinePolicy.MinSamples = 1
	now := time.Now().UTC()
	before := baselineProcessed(t, "skipped")
	m.refreshBaselines(now)
	if baselineProcessed(t, "skipped") <= before {
		t.Fatal("insufficient samples not counted")
	}
	if err := store.BatchInsertChecks([]db.CheckResult{{MonitorID: "m1", Status: "up", Latency: 10, Timestamp: now}}); err != nil {
		t.Fatal(err)
	}
	before = baselineProcessed(t, "updated")
	m.refreshBaselines(now)
	if baselineProcessed(t, "updated") != before+1 {
		t.Fatal("updated monitor not counted")
	}
}
