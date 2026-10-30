package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStartupHandler(t *testing.T) {
	h := &startupHandler{}
	get := func(path string) int {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		return w.Code
	}

	// Before the router is ready: alive, not ready, nothing else served.
	for path, want := range map[string]int{
		"/healthz":   http.StatusOK,
		"/readyz":    http.StatusServiceUnavailable,
		"/api/s/all": http.StatusServiceUnavailable,
		"/":          http.StatusServiceUnavailable,
	} {
		if got := get(path); got != want {
			t.Errorf("starting: GET %s = %d, want %d", path, got, want)
		}
	}

	h.ready(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	for _, path := range []string{"/healthz", "/readyz", "/"} {
		if got := get(path); got != http.StatusTeapot {
			t.Errorf("ready: GET %s = %d, want the router's %d", path, got, http.StatusTeapot)
		}
	}
}
