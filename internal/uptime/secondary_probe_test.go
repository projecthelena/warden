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
