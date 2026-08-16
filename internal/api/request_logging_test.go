package api

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestRequestLogger_ProbeLogging(t *testing.T) {
	var buf bytes.Buffer
	orig := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(orig)

	r := chi.NewRouter()
	r.Use(requestLogger)
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})
	r.Get("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "unavailable"})
	})

	serve := func(path string) {
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", path, nil))
	}

	// A healthy probe is silent.
	serve("/healthz")
	if strings.Contains(buf.String(), "/healthz") {
		t.Errorf("healthy probe should not be logged, got: %q", buf.String())
	}

	// A failing readiness probe is logged, since that is the case worth seeing.
	buf.Reset()
	serve("/readyz")
	if !strings.Contains(buf.String(), "/readyz") {
		t.Errorf("failing probe should be logged, got: %q", buf.String())
	}
	if !strings.Contains(buf.String(), "503") {
		t.Errorf("failing probe log should carry the status, got: %q", buf.String())
	}
}
