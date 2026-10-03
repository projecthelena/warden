package observability

import (
	"net/http"
	"net/http/httptest"
	"testing"

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
