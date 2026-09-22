package observability

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/testutil"
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
	beforeSuccess := testutil.ToFloat64(rollupRuns.WithLabelValues(mode, "success"))
	beforeError := testutil.ToFloat64(rollupRuns.WithLabelValues(mode, "error"))

	SetRollupInProgress(mode, true)
	if got := testutil.ToFloat64(rollupInProgress.WithLabelValues(mode)); got != 1 {
		t.Fatalf("in-progress gauge = %v, want 1", got)
	}

	ObserveRollup(mode, time.Second, 2*time.Second, 3*time.Second, 12, nil)
	ObserveRollup(mode, time.Second, 0, time.Second, 0, errors.New("rollup failed"))
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
}
