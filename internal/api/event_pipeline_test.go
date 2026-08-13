package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/projecthelena/warden/internal/db"
	"github.com/projecthelena/warden/internal/uptime"
	"github.com/go-chi/chi/v5"
)

// TestEventPipeline_RealisticTimeout simulates exactly the user's situation:
// a network-timeout failure produces an enriched monitor_events row, and the
// /api/monitors/:id/events?date= endpoint returns it with the shape the UI needs.
func TestEventPipeline_RealisticTimeout(t *testing.T) {
	s, _ := db.NewStore(db.NewTestConfig())
	m := uptime.NewManager(s)
	h := NewUptimeHandler(m, s)

	_ = s.CreateGroup(db.Group{ID: "g1", Name: "Prod"})
	_ = s.CreateMonitor(db.Monitor{ID: "github", GroupID: "g1", Name: "GitHub", URL: "https://github.com", Interval: 60})

	// Simulate exactly what the resultProcessor writes for a timeout: no statusCode,
	// no body, no headers — but a populated errorMessage and latency.
	if err := s.CreateEventWithDetails("github", "down", "Monitor is down", &db.EventDetails{
		Latency:      5001,
		ErrorMessage: `Get "https://github.com": context deadline exceeded (Client.Timeout exceeded while awaiting headers)`,
	}); err != nil {
		t.Fatalf("CreateEventWithDetails: %v", err)
	}

	// And a 502 response from a real server — has status, body, headers.
	if err := s.CreateEventWithDetails("github", "down", "Monitor is down (Status: 502)", &db.EventDetails{
		StatusCode:      502,
		Latency:         812,
		ErrorMessage:    "",
		ResponseBody:    `<html><head><title>502 Bad Gateway</title></head><body>upstream timed out</body></html>`,
		ResponseHeaders: `{"content-type":"text/html","server":"nginx/1.25","x-request-id":"abc123"}`,
	}); err != nil {
		t.Fatalf("CreateEventWithDetails: %v", err)
	}

	today := time.Now().UTC().Format("2006-01-02")

	r := chi.NewRouter()
	r.Get("/api/monitors/{id}/events", h.GetMonitorEvents)

	req := httptest.NewRequest("GET", "/api/monitors/github/events?date="+today+"&limit=10", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var events []MonitorEventDTO
	if err := json.Unmarshal(w.Body.Bytes(), &events); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	// Find the 502 and the timeout — they should both serialize cleanly.
	var timeoutEvt, badGatewayEvt MonitorEventDTO
	var foundTimeout, foundBadGateway bool
	for _, e := range events {
		if e.StatusCode != nil && *e.StatusCode == 502 {
			badGatewayEvt = e
			foundBadGateway = true
		} else if e.ErrorMessage != nil {
			timeoutEvt = e
			foundTimeout = true
		}
	}

	if !foundTimeout {
		t.Fatalf("missing timeout event in %+v", events)
	}
	if !foundBadGateway {
		t.Fatalf("missing 502 event in %+v", events)
	}

	// Timeout: error populated, no body/headers/status
	if timeoutEvt.Latency == nil || *timeoutEvt.Latency != 5001 {
		t.Errorf("timeout: latency = %v, want 5001", timeoutEvt.Latency)
	}
	if timeoutEvt.StatusCode != nil {
		t.Errorf("timeout: status code should be nil, got %v", *timeoutEvt.StatusCode)
	}
	if timeoutEvt.ResponseBody != nil {
		t.Errorf("timeout: response body should be nil, got %q", *timeoutEvt.ResponseBody)
	}
	if timeoutEvt.ResponseHeaders != nil {
		t.Errorf("timeout: response headers should be nil, got %v", timeoutEvt.ResponseHeaders)
	}

	// 502: full enriched payload
	if badGatewayEvt.StatusCode == nil || *badGatewayEvt.StatusCode != 502 {
		t.Errorf("502: status = %v", badGatewayEvt.StatusCode)
	}
	if badGatewayEvt.ResponseBody == nil || *badGatewayEvt.ResponseBody == "" {
		t.Errorf("502: response body should be populated")
	}
	if badGatewayEvt.ResponseHeaders == nil {
		t.Fatalf("502: response headers should be unmarshalled into map")
	}
	if ct := badGatewayEvt.ResponseHeaders["content-type"]; ct != "text/html" {
		t.Errorf("502: content-type header = %q, want text/html", ct)
	}
	if srv := badGatewayEvt.ResponseHeaders["server"]; srv != "nginx/1.25" {
		t.Errorf("502: server header = %q, want nginx/1.25", srv)
	}

	// Print the actual JSON the frontend will receive — visual sanity check.
	fmt.Printf("\n----- API response (frontend EnrichedMonitorEvent[]) -----\n%s\n----------\n", w.Body.String())
}

// TestGetMonitorEvents_RangeValidation guards the from/to RFC3339 range path on
// /api/monitors/:id/events: both must be present together, both must parse, and
// `from` must be strictly before `to`. Without these checks a swapped client call
// would silently return [] and look like a working "no events" state.
func TestGetMonitorEvents_RangeValidation(t *testing.T) {
	s, _ := db.NewStore(db.NewTestConfig())
	mgr := uptime.NewManager(s)
	h := NewUptimeHandler(mgr, s)
	_ = s.CreateGroup(db.Group{ID: "g1", Name: "G"})
	_ = s.CreateMonitor(db.Monitor{ID: "m1", GroupID: "g1", Name: "M", Interval: 60})

	r := chi.NewRouter()
	r.Get("/api/monitors/{id}/events", h.GetMonitorEvents)

	cases := []struct {
		name, url string
		want      int
	}{
		{"from without to", "/api/monitors/m1/events?from=2026-05-17T00:00:00Z", 400},
		{"to without from", "/api/monitors/m1/events?to=2026-05-17T01:00:00Z", 400},
		{"invalid from", "/api/monitors/m1/events?from=not-rfc3339&to=2026-05-17T01:00:00Z", 400},
		{"from after to", "/api/monitors/m1/events?from=2026-05-17T02:00:00Z&to=2026-05-17T01:00:00Z", 400},
		{"from equal to", "/api/monitors/m1/events?from=2026-05-17T01:00:00Z&to=2026-05-17T01:00:00Z", 400},
		{"valid range", "/api/monitors/m1/events?from=2026-05-17T00:00:00Z&to=2026-05-17T01:00:00Z", 200},
	}
	for _, c := range cases {
		req := httptest.NewRequest("GET", c.url, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != c.want {
			t.Errorf("%s: expected %d, got %d body=%s", c.name, c.want, w.Code, w.Body.String())
		}
	}
}
