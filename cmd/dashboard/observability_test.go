package main

import (
	"net/http/httptest"
	"testing"
)

func TestObservabilityMetricsSwitch(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		handler := observabilityHandler(enabled)
		for _, path := range []string{"/metrics", "/debug/pprof/"} {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
			want := 200
			if path == "/metrics" && !enabled {
				want = 404
			}
			if rec.Code != want {
				t.Errorf("enabled=%v path=%s: got %d want %d", enabled, path, rec.Code, want)
			}
		}
	}
}
