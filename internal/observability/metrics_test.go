package observability

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/projecthelena/warden/internal/db"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
)

func TestHTTPMiddlewareUsesRoutePattern(t *testing.T) {
	r := chi.NewRouter()
	r.Use(HTTPMiddleware)
	r.Get("/monitors/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	before := testutil.ToFloat64(httpRequests.WithLabelValues("/monitors/{id}", http.MethodGet, "204"))
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/monitors/secret-monitor-id", nil))
	after := testutil.ToFloat64(httpRequests.WithLabelValues("/monitors/{id}", http.MethodGet, "204"))

	if after-before != 1 {
		t.Fatalf("request counter delta = %v, want 1", after-before)
	}
}

func TestObserveRollupRecordsBoundedOutcomeAndProgress(t *testing.T) {
	mode := "test"
	rollupDuration.Reset()
	beforeSuccess := testutil.ToFloat64(rollupRuns.WithLabelValues(mode, "success"))
	beforeError := testutil.ToFloat64(rollupRuns.WithLabelValues(mode, "error"))

	SetRollupInProgress(mode, true)
	if got := testutil.ToFloat64(rollupInProgress.WithLabelValues(mode)); got != 1 {
		t.Fatalf("in-progress gauge = %v, want 1", got)
	}

	ObserveRollup(mode, db.DailyUptimeRollupStats{QueryDuration: time.Second, UpsertDuration: 2 * time.Second, Rows: 12}, 3*time.Second, nil)
	ObserveRollup(mode, db.DailyUptimeRollupStats{QueryDuration: time.Second}, time.Second, errors.New("rollup failed"))
	SetRollupInProgress(mode, false)

	if got := testutil.ToFloat64(rollupRuns.WithLabelValues(mode, "success")) - beforeSuccess; got != 1 {
		t.Fatalf("success counter delta = %v, want 1", got)
	}
	if got := testutil.ToFloat64(rollupRuns.WithLabelValues(mode, "error")) - beforeError; got != 1 {
		t.Fatalf("error counter delta = %v, want 1", got)
	}
	if got := testutil.ToFloat64(rollupInProgress.WithLabelValues(mode)); got != 0 {
		t.Fatalf("in-progress gauge = %v, want 0", got)
	}
	for phase, seconds := range map[string]float64{
		"read_connection": 0, "query": 2, "scan": 0, "write_connection": 0,
		"begin": 0, "upsert": 2, "commit": 0, "total": 4,
	} {
		metric := &dto.Metric{}
		if err := rollupDuration.WithLabelValues(mode, phase).(prometheus.Metric).Write(metric); err != nil {
			t.Fatal(err)
		}
		if metric.Histogram.GetSampleCount() != 2 || metric.Histogram.GetSampleSum() != seconds {
			t.Errorf("phase %s: got %v", phase, metric.Histogram)
		}
	}
}
