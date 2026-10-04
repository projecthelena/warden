package uptime

import (
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/projecthelena/warden/internal/db"
)

func TestAutomaticRetryRecoversWithoutHidingFirstFailure(t *testing.T) {
	var calls atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer s.Close()
	transport := checkTransport()
	defer transport.CloseIdleConnections()
	r := runCheck(Job{URL: s.URL, RequestConfig: &db.RequestConfig{AutoRetry: true}}, transport)
	if !r.Status || calls.Load() != 2 || r.Diagnostics == nil || len(r.Diagnostics.Attempts) != 2 {
		t.Fatalf("result %+v calls %d", r, calls.Load())
	}
	if r.Diagnostics.Attempts[0].Status != "down" || r.Diagnostics.Attempts[1].Status != "up" || r.Diagnostics.RetryMode != "automatic" {
		t.Fatal("lost initial failure")
	}
}

func TestAutomaticRetryStopsOnPermanentErrorsAndUnsafeMethods(t *testing.T) {
	for _, tc := range []struct {
		name, method, header string
		code                 int
	}{{"post", "POST", "", 503}, {"auth", "GET", "", 401}, {"missing", "GET", "", 404}, {"rate_limit", "GET", "", 429}, {"server_bug", "GET", "", 500}, {"retry_after", "GET", "30", 503}} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				if tc.header != "" {
					w.Header().Set("Retry-After", tc.header)
				}
				w.WriteHeader(tc.code)
			}))
			defer s.Close()
			transport := checkTransport()
			defer transport.CloseIdleConnections()
			r := runCheck(Job{URL: s.URL, RequestConfig: &db.RequestConfig{AutoRetry: true, Method: tc.method}}, transport)
			if r.Status || calls.Load() != 1 || r.Diagnostics == nil {
				t.Fatalf("unexpected retry: %+v %d", r, calls.Load())
			}
		})
	}
}

func TestAutomaticRetryUsesOneCheckBudget(t *testing.T) {
	var calls atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { calls.Add(1); <-r.Context().Done() }))
	defer s.Close()
	transport := checkTransport()
	defer transport.CloseIdleConnections()
	started := time.Now()
	r := runCheck(Job{URL: s.URL, RequestConfig: &db.RequestConfig{AutoRetry: true, TimeoutSeconds: 1}}, transport)
	if r.Status || calls.Load() != 1 || r.Diagnostics.RetryDecision != "budget_exhausted" || time.Since(started) > 2*time.Second {
		t.Fatalf("budget not bounded: %+v", r.Diagnostics)
	}
}

func TestAutomaticRetryClassification(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want bool
	}{{io.EOF, true}, {&net.OpError{Op: "dial", Err: errors.New("refused")}, true}, {&net.DNSError{IsNotFound: true}, false}, {&net.DNSError{IsTimeout: true}, true}, {&tls.CertificateVerificationError{Err: errors.New("invalid")}, false}, {errBlockedTarget, false}} {
		if got := transientHTTPFailure(checkOutcome{cause: tc.err}); got != tc.want {
			t.Fatalf("%T retry=%v", tc.err, got)
		}
	}
}

func TestAutomaticRecoveryDoesNotOpenOutage(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusBadGateway)
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	one := 1
	if err := store.CreateMonitor(db.Monitor{ID: "m-retry", GroupID: "g-default", Name: "Retry", URL: srv.URL, Interval: 60, Active: true, ConfirmationThreshold: &one, RequestConfig: &db.RequestConfig{AutoRetry: true}}); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(store)
	manager.Start()
	manager.Sync()
	defer manager.Stop()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		checks, err := store.GetMonitorChecks("m-retry", 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(checks) > 0 {
			if checks[0].Status != "up" || checks[0].Diagnostics == nil || len(checks[0].Diagnostics.Attempts) != 2 {
				t.Fatalf("lost recovery: %+v", checks[0])
			}
			outages, err := store.GetOpenOutages()
			if err != nil || len(outages) != 0 {
				t.Fatalf("false outage: %+v %v", outages, err)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("recovered check was not persisted")
}

func TestAutomaticRetryDoesNotWaitForCapacity(t *testing.T) {
	var calls atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls.Add(1); w.WriteHeader(http.StatusBadGateway) }))
	defer s.Close()
	slots := make(chan struct{}, 1)
	slots <- struct{}{}
	transport := checkTransport()
	defer transport.CloseIdleConnections()
	r := runCheck(Job{URL: s.URL, retrySlots: slots, RequestConfig: &db.RequestConfig{AutoRetry: true}}, transport)
	if calls.Load() != 1 || r.Diagnostics.RetryDecision != "capacity_limited" {
		t.Fatalf("retry ignored capacity: %+v", r.Diagnostics)
	}
	if len(slots) != 1 {
		t.Fatal("released an unowned slot")
	}
}

func TestRetryPolicyBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name      string
		cfg       db.RequestConfig
		busy      bool
		wantCalls int32
		wantUp    bool
		decision  string
	}{
		{"persistent transient stops after two", db.RequestConfig{AutoRetry: true}, false, 2, false, "retried"},
		{"HEAD retries", db.RequestConfig{AutoRetry: true, Method: "HEAD"}, false, 2, false, "retried"},
		{"GET body is not replayed", db.RequestConfig{AutoRetry: true, Body: "payload"}, false, 1, false, "unsafe_request"},
		{"PUT is not replayed", db.RequestConfig{AutoRetry: true, Method: "PUT"}, false, 1, false, "unsafe_request"},
		{"DELETE is not replayed", db.RequestConfig{AutoRetry: true, Method: "DELETE"}, false, 1, false, "unsafe_request"},
		{"queue pressure skips retry", db.RequestConfig{AutoRetry: true}, true, 1, false, "capacity_limited"},
		{"accepted error is success", db.RequestConfig{AutoRetry: true, AcceptedStatusCodes: "503"}, false, 1, true, ""},
		{"manual policy is preserved", db.RequestConfig{RetryCount: 2}, false, 3, false, "retried"},
		{"explicit opt out", db.RequestConfig{}, false, 1, false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int32
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusServiceUnavailable)
			}))
			defer s.Close()
			transport := checkTransport()
			defer transport.CloseIdleConnections()
			slots := make(chan struct{}, 1)
			r := runCheck(Job{URL: s.URL, CaptureDiagnostics: true, RequestConfig: &tc.cfg, retryBusy: tc.busy, retrySlots: slots}, transport)
			if calls.Load() != tc.wantCalls || r.Status != tc.wantUp || r.Diagnostics == nil {
				t.Fatalf("calls=%d result=%+v", calls.Load(), r)
			}
			if len(r.Diagnostics.Attempts) != int(tc.wantCalls) || r.Diagnostics.RetryDecision != tc.decision {
				t.Fatalf("lost policy evidence: %+v", r.Diagnostics)
			}
			if len(slots) != 0 {
				t.Fatal("retry capacity leaked")
			}
		})
	}
}
