package uptime

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/projecthelena/warden/internal/db"
	"github.com/projecthelena/warden/internal/metricstate"
	"github.com/projecthelena/warden/internal/observability"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
)

func histogramCount(t *testing.T, observer prometheus.Observer) uint64 {
	t.Helper()
	var metric dto.Metric
	if err := observer.(prometheus.Metric).Write(&metric); err != nil {
		t.Fatal(err)
	}
	return metric.GetHistogram().GetSampleCount()
}

func TestWorkerMetricsSwitchForBothOutcomes(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		for _, healthy := range []bool{true, false} {
			t.Run(fmt.Sprintf("enabled=%t/healthy=%t", enabled, healthy), func(t *testing.T) {
				metricstate.Configure(enabled)
				t.Cleanup(func() { metricstate.Configure(true) })
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					if !healthy {
						w.WriteHeader(http.StatusInternalServerError)
					}
				}))
				defer server.Close()
				m, _ := newAlertTestManager(t)
				outcome := "up"
				if !healthy {
					outcome = "down"
				}
				counter := observability.Checks.WithLabelValues(db.MonitorTypeHTTP, outcome)
				duration := observability.CheckDuration.WithLabelValues(db.MonitorTypeHTTP)
				before, beforeDuration := testutil.ToFloat64(counter), histogramCount(t, duration)
				m.jobQueue <- Job{MonitorID: "m1", Type: db.MonitorTypeHTTP, URL: server.URL}
				close(m.jobQueue)
				m.wg.Add(1)
				m.worker()
				result := <-m.resultQueue
				if result.Status != healthy {
					t.Fatalf("status=%v", result.Status)
				}
				var delta uint64
				if enabled {
					delta = 1
				}
				if testutil.ToFloat64(counter) != before+float64(delta) {
					t.Fatal("incorrect check counter")
				}
				if histogramCount(t, duration) != beforeDuration+delta {
					t.Fatal("incorrect check duration count")
				}
			})
		}
	}
}

func TestPersistMetricsSwitchForBothOutcomes(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		for _, failed := range []bool{true, false} {
			t.Run(fmt.Sprintf("enabled=%t/failed=%t", enabled, failed), func(t *testing.T) {
				metricstate.Configure(enabled)
				t.Cleanup(func() { metricstate.Configure(true) })
				m, store := newAlertTestManager(t)
				// Only exercise persistence; no running monitor means no alert side effects.
				m.monitors = map[string]*Monitor{}
				m.resultQueue = make(chan CheckResult)
				outcome := "success"
				if failed {
					outcome = "error"
					if err := store.Close(); err != nil {
						t.Fatal(err)
					}
				}
				duration := observability.PersistDuration.WithLabelValues(outcome)
				before := histogramCount(t, duration)
				m.wg.Add(1)
				go m.resultProcessor()
				m.resultQueue <- CheckResult{MonitorID: "m1", Status: true, Timestamp: time.Now().UTC()}
				close(m.resultQueue)
				m.wg.Wait()
				var delta uint64
				if enabled {
					delta = 1
				}
				if histogramCount(t, duration) != before+delta {
					t.Fatal("incorrect persist duration count")
				}
				if !failed {
					checks, err := store.GetMonitorChecks("m1", 10)
					if err != nil || len(checks) != 1 {
						t.Fatalf("checks=%d err=%v", len(checks), err)
					}
				}
			})
		}
	}
}
