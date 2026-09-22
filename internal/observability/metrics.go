package observability

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
)

// RegisterDatabaseStats exposes database/sql pool saturation without coupling the store to
// Prometheus. It must be called once during process startup.
func RegisterDatabaseStats(dialect string, stats func() sql.DBStats) {
	register := func(name, help string, value func(sql.DBStats) float64) {
		prometheus.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name:        name,
			Help:        help,
			ConstLabels: prometheus.Labels{"dialect": dialect},
		}, func() float64 { return value(stats()) }))
	}

	register("warden_db_connections_open", "Open database connections.", func(s sql.DBStats) float64 { return float64(s.OpenConnections) })
	register("warden_db_connections_in_use", "Database connections currently in use.", func(s sql.DBStats) float64 { return float64(s.InUse) })
	register("warden_db_connections_idle", "Idle database connections.", func(s sql.DBStats) float64 { return float64(s.Idle) })
	register("warden_db_connections_max", "Configured maximum number of open database connections.", func(s sql.DBStats) float64 { return float64(s.MaxOpenConnections) })
	register("warden_db_wait_total", "Total waits for a database connection.", func(s sql.DBStats) float64 { return float64(s.WaitCount) })
	register("warden_db_wait_duration_seconds_total", "Total time blocked waiting for a database connection.", func(s sql.DBStats) float64 { return s.WaitDuration.Seconds() })
}

var (
	httpInFlight = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "warden_http_requests_in_flight",
		Help: "Number of HTTP requests currently being served.",
	})
	httpRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "warden_http_requests_total",
		Help: "HTTP requests completed, partitioned by route, method, and status code.",
	}, []string{"route", "method", "status"})
	httpDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "warden_http_request_duration_seconds",
		Help:    "HTTP request duration by route and method.",
		Buckets: prometheus.DefBuckets,
	}, []string{"route", "method"})
	Checks = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "warden_monitor_checks_total",
		Help: "Monitor checks completed, partitioned by monitor type and outcome.",
	}, []string{"type", "outcome"})
	CheckScheduling = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "warden_monitor_check_scheduling_total",
		Help: "Monitor scheduling attempts partitioned by queued or dropped outcome.",
	}, []string{"outcome"})
	CheckDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "warden_monitor_check_duration_seconds",
		Help:    "End-to-end monitor check duration by monitor type.",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 30},
	}, []string{"type"})
	CheckQueueDepth = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "warden_monitor_check_queue_depth",
		Help: "Number of monitor checks waiting for a worker.",
	})
	ResultQueueDepth = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "warden_monitor_result_queue_depth",
		Help: "Number of completed checks waiting to be processed.",
	})
	PersistBatchSize = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "warden_monitor_persist_batch_size",
		Help:    "Number of check results in each database write batch.",
		Buckets: prometheus.LinearBuckets(1, 5, 11),
	})
	PersistDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "warden_monitor_persist_duration_seconds",
		Help:    "Duration of check-result batch writes by outcome.",
		Buckets: prometheus.DefBuckets,
	}, []string{"outcome"})
	ActiveMonitors = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "warden_monitors_active",
		Help: "Number of active monitors managed by this Warden instance.",
	})
	StatusPagePhase = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "warden_status_page_phase_duration_seconds",
		Help:    "Status page response construction time partitioned by bounded phase name.",
		Buckets: prometheus.DefBuckets,
	}, []string{"phase"})
	StatusPagePayloadBytes = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "warden_status_page_payload_bytes",
		Help:    "Uncompressed status page JSON payload size partitioned by response view.",
		Buckets: prometheus.ExponentialBuckets(1024, 2, 15),
	}, []string{"view"})
	rollupInProgress = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "warden_uptime_rollup_in_progress",
		Help: "Whether a daily uptime rollup is currently running, partitioned by mode.",
	}, []string{"mode"})
	rollupRuns = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "warden_uptime_rollup_runs_total",
		Help: "Daily uptime rollup runs partitioned by mode and outcome.",
	}, []string{"mode", "outcome"})
	rollupDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "warden_uptime_rollup_duration_seconds",
		Help:    "Daily uptime rollup duration partitioned by mode and bounded phase.",
		Buckets: []float64{.01, .025, .05, .1, .25, .5, 1, 2.5, 3, 5, 10, 30, 60},
	}, []string{"mode", "phase"})
	rollupRows = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "warden_uptime_rollup_rows",
		Help:    "Number of monitor-day rows produced by a daily uptime rollup.",
		Buckets: prometheus.ExponentialBuckets(1, 2, 13),
	}, []string{"mode"})
)

func init() {
	prometheus.MustRegister(
		httpInFlight, httpRequests, httpDuration,
		Checks, CheckScheduling, CheckDuration, CheckQueueDepth, ResultQueueDepth,
		PersistBatchSize, PersistDuration, ActiveMonitors,
		StatusPagePhase, StatusPagePayloadBytes,
		rollupInProgress, rollupRuns, rollupDuration, rollupRows,
	)
}

// ObserveRollup records one completed rollup without exposing unbounded labels.
func ObserveRollup(mode string, aggregation, upsert, total time.Duration, rows int, err error) {
	outcome := "success"
	if err != nil {
		outcome = "error"
	}
	rollupRuns.WithLabelValues(mode, outcome).Inc()
	rollupDuration.WithLabelValues(mode, "aggregation").Observe(aggregation.Seconds())
	rollupDuration.WithLabelValues(mode, "upsert").Observe(upsert.Seconds())
	rollupDuration.WithLabelValues(mode, "total").Observe(total.Seconds())
	rollupRows.WithLabelValues(mode).Observe(float64(rows))
}

// SetRollupInProgress exposes the contention window to scrapers while a rollup runs.
func SetRollupInProgress(mode string, running bool) {
	value := 0.0
	if running {
		value = 1
	}
	rollupInProgress.WithLabelValues(mode).Set(value)
}

// HTTPMiddleware records bounded-cardinality route templates, never raw URLs.
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		httpInFlight.Inc()
		defer httpInFlight.Dec()

		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)

		route := chi.RouteContext(r.Context()).RoutePattern()
		if route == "" {
			route = "unmatched"
		}
		statusCode := ww.Status()
		if statusCode == 0 {
			statusCode = http.StatusOK
		}
		status := strconv.Itoa(statusCode)
		httpRequests.WithLabelValues(route, r.Method, status).Inc()
		httpDuration.WithLabelValues(route, r.Method).Observe(time.Since(start).Seconds())
	})
}
