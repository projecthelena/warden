package uptime

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/projecthelena/warden/internal/db"
)

func TestSecondaryProbeRegisteredTargetsAndAuthentication(t *testing.T) {
	var calls atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "" {
			t.Error("probe token forwarded to target")
		}
		w.WriteHeader(503)
	}))
	defer target.Close()
	token := strings.Repeat("x", 32)
	h, err := NewProbeHandler(token, map[string]string{"m-test": target.URL})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(h)
	defer server.Close()
	for _, path := range []string{"/v1/check/m-test", "/v1/check/unknown?url=" + target.URL} {
		r := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 401 {
			t.Fatal("unauthenticated probe")
		}
	}
	r := httptest.NewRequest("GET", "/v1/check/unknown", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 404 || calls.Load() != 0 {
		t.Fatal("arbitrary target accepted")
	}
	probe, err := NewSecondaryProbe(server.URL, token)
	if err != nil {
		t.Fatal(err)
	}
	o := probe.Observe("m-test")
	if o.Outcome != "response" || o.StatusCode != 503 || calls.Load() != 1 {
		t.Fatalf("observation: %+v", o)
	}
	if probe.Observe("m-test").Outcome != "rate_limited" || calls.Load() != 1 {
		t.Fatal("missing burst bound")
	}
}

func TestSecondaryProbeUnavailableStaleAndUnsafeEndpoints(t *testing.T) {
	for _, endpoint := range []string{"http://example.test", "ftp://example.test", "https://user:password@example.test", "https://example.test?token=secret"} {
		if _, err := NewSecondaryProbe(endpoint, strings.Repeat("x", 32)); err == nil {
			t.Fatalf("accepted %s", endpoint)
		}
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(db.ExternalObservation{Outcome: "response", StatusCode: 200, ObservedAt: time.Now().Add(-time.Hour).Format(time.RFC3339Nano)})
	}))
	defer s.Close()
	p, _ := NewSecondaryProbe(s.URL, strings.Repeat("x", 32))
	if p.Observe("m").Outcome != "unavailable" {
		t.Fatal("trusted stale observation")
	}
}

func TestConnectivityEvidenceNeedsRecentDistinctDestinations(t *testing.T) {
	now := time.Now()
	m := &Manager{monitors: make(map[string]*Monitor), jobQueue: make(chan Job, 10)}
	outages := []db.OpenOutage{}
	for _, id := range []string{"a", "b", "c"} {
		m.monitors[id] = &Monitor{diagnosticAt: now, lastDiagnostics: &db.HTTPDiagnostics{Attempts: []db.HTTPAttempt{{Hops: []db.HTTPHop{{Host: id + ".example.test", FailurePhase: "tcp"}}}}}}
		outages = append(outages, db.OpenOutage{MonitorID: id, StartTime: now})
	}
	if !strings.Contains(m.connectivityEvidence(outages, now), "Possible connectivity problem") {
		t.Fatal("missing corroboration")
	}
	m.monitors["c"].lastDiagnostics.Attempts[0].Hops[0].Host = "a.example.test"
	if strings.Contains(m.connectivityEvidence(outages, now), "Possible connectivity problem") {
		t.Fatal("duplicate host counted")
	}
	m.monitors["c"].lastDiagnostics.Attempts[0].Hops[0].Host = "c.example.test"
	m.monitors["c"].diagnosticAt = now.Add(-10 * time.Minute)
	if strings.Contains(m.connectivityEvidence(outages, now), "Possible connectivity problem") {
		t.Fatal("stale check counted")
	}
	for i := 0; i < 5; i++ {
		m.jobQueue <- Job{}
	}
	if !strings.Contains(m.connectivityEvidence(outages, now), "queue pressure") {
		t.Fatal("local health ignored")
	}
}

func TestConnectivityEvidenceDoesNotJoinUnrelatedOutages(t *testing.T) {
	now := time.Now()
	m := &Manager{monitors: map[string]*Monitor{}}
	var outages []db.OpenOutage
	for i, id := range []string{"a", "b", "c"} {
		m.monitors[id] = &Monitor{diagnosticAt: now, lastDiagnostics: &db.HTTPDiagnostics{Attempts: []db.HTTPAttempt{{Hops: []db.HTTPHop{{Host: id + ".example.test", FailurePhase: "dns"}}}}}}
		outages = append(outages, db.OpenOutage{MonitorID: id, StartTime: now.Add(-time.Duration(i) * time.Hour)})
	}
	if strings.Contains(m.connectivityEvidence(outages, now), "Possible connectivity problem") {
		t.Fatal("joined failures an hour apart")
	}
}

func TestSecondaryProbeRejectsInvalidObservations(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
	}{
		{"malformed", `{`, 200},
		{"missing timestamp", `{"outcome":"response","statusCode":200}`, 200},
		{"future", `{"outcome":"response","statusCode":200,"observedAt":"2099-01-01T00:00:00Z"}`, 200},
		{"bad status", `{"outcome":"response","statusCode":999,"observedAt":"NOW"}`, 200},
		{"unknown outcome", `{"outcome":"healthy","observedAt":"NOW"}`, 200},
		{"unknown phase", `{"outcome":"failed","failurePhase":"secret","observedAt":"NOW"}`, 200},
		{"oversized", `{"outcome":"` + strings.Repeat("x", 4096) + `"}`, 200},
		{"unauthorized", `{}`, 401},
		{"busy", `{}`, 429},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(strings.ReplaceAll(tc.body, "NOW", time.Now().UTC().Format(time.RFC3339Nano))))
			}))
			defer s.Close()
			p, err := NewSecondaryProbe(s.URL, strings.Repeat("t", 32))
			if err != nil {
				t.Fatal(err)
			}
			defer p.client.CloseIdleConnections()
			if got := p.Observe("registered"); got.Outcome != "unavailable" {
				t.Fatalf("accepted invalid evidence: %+v", got)
			}
			if len(p.slots) != 0 {
				t.Fatal("probe capacity leaked")
			}
		})
	}
}

func TestSecondaryProbeDoesNotFollowRedirects(t *testing.T) {
	calls := make(chan struct{}, 1)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls <- struct{}{}; w.WriteHeader(200) }))
	defer target.Close()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
	defer s.Close()
	p, err := NewSecondaryProbe(s.URL, strings.Repeat("t", 32))
	if err != nil {
		t.Fatal(err)
	}
	defer p.client.CloseIdleConnections()
	if got := p.Observe("registered"); got.Outcome != "unavailable" {
		t.Fatalf("unexpected result: %+v", got)
	}
	select {
	case <-calls:
		t.Fatal("followed redirect with probe credentials")
	default:
	}
}

func TestProbeHandlerCapacityAndMethod(t *testing.T) {
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		entered <- struct{}{}
		<-release
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()
	token := strings.Repeat("t", 32)
	h, err := NewProbeHandler(token, map[string]string{"registered": target.URL})
	if err != nil {
		t.Fatal(err)
	}
	invoke := func(method string) int {
		r := httptest.NewRequest(method, "/v1/check/registered", nil)
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	if code := invoke(http.MethodPost); code != http.StatusMethodNotAllowed {
		t.Fatalf("POST: %d", code)
	}
	done := make(chan int, 2)
	defer func() { close(release); <-done; <-done }()
	for range 2 {
		go func() { done <- invoke(http.MethodGet) }()
	}
	for range 2 {
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("target did not receive two concurrent checks")
		}
	}
	if code := invoke(http.MethodGet); code != http.StatusTooManyRequests {
		t.Fatalf("third check should be rejected: %d", code)
	}
}
