// Package operationmetrics records bounded database and maintenance operations.
package operationmetrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Operation values are fixed names, never SQL, URLs, or monitor identifiers.
type Operation string

const (
	BaselineCount     Operation = "baseline_count"
	BaselineP50       Operation = "baseline_p50"
	BaselineP95       Operation = "baseline_p95"
	BaselineSave      Operation = "baseline_save"
	BaselineCycle     Operation = "baseline_cycle"
	PersistConnection Operation = "persist_connection"
	PersistBegin      Operation = "persist_begin"
	PersistInsert     Operation = "persist_insert"
	PersistCommit     Operation = "persist_commit"
)

var (
	duration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "warden_db_operation_duration_seconds",
		Help:    "Wall time of database operations and baseline cycles, including connection waits where applicable.",
		Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 30, 60, 120, 300, 600, 1800},
	}, []string{"operation", "outcome"})
	inFlight = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "warden_db_operations_in_flight", Help: "Operations currently running, including blocked operations.",
	}, []string{"operation"})
	lastStarted = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "warden_db_operation_last_started_timestamp_seconds", Help: "Unix timestamp of the most recently started operation; with overlapping calls this is not the oldest active call.",
	}, []string{"operation"})
	BaselineMonitors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "warden_baseline_monitors_total", Help: "Baseline computations by updated, skipped, or error outcome.",
	}, []string{"outcome"})
)

func init() {
	prometheus.MustRegister(duration, inFlight, lastStarted, BaselineMonitors)
}

// Start records active work immediately; finish must be called exactly once.
func Start(operation Operation) func(error) {
	label := string(operation)
	started := time.Now()
	inFlight.WithLabelValues(label).Inc()
	lastStarted.WithLabelValues(label).Set(float64(started.UnixNano()) / 1e9)
	return func(err error) {
		outcome := "success"
		if err != nil {
			outcome = "error"
		}
		duration.WithLabelValues(label, outcome).Observe(time.Since(started).Seconds())
		inFlight.WithLabelValues(label).Dec()
	}
}
