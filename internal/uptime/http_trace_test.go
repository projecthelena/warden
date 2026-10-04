package uptime

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/projecthelena/warden/internal/db"
)

func TestHTTPDiagnosticsRedirectRetryAndOptOut(t *testing.T) {
	var calls atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/end?secret=hidden", http.StatusFound)
			return
		}
		if calls.Add(1) == 1 {
			w.WriteHeader(503)
		} else {
			w.WriteHeader(204)
		}
	}))
	defer s.Close()
	transport := checkTransport()
	defer transport.CloseIdleConnections()
	result := runCheck(Job{URL: s.URL + "/start", CaptureDiagnostics: true, RequestConfig: &db.RequestConfig{RetryCount: 1}}, transport)
	if !result.Status || result.Diagnostics == nil || len(result.Diagnostics.Attempts) != 2 {
		t.Fatalf("result: %+v", result)
	}
	for _, a := range result.Diagnostics.Attempts {
		if len(a.Hops) != 2 {
			t.Fatalf("redirects not separated: %+v", a)
		}
		for _, h := range a.Hops {
			if strings.Contains(h.Host, "secret") || h.RemoteIP != "127.0.0.1" || h.Reused == nil {
				t.Fatalf("bad hop: %+v", h)
			}
		}
	}
	if result.Diagnostics.Attempts[0].Hops[1].StatusCode != 503 || result.Diagnostics.Attempts[1].Hops[1].StatusCode != 204 {
		t.Fatal("lost retry status")
	}
	off := runCheck(Job{URL: s.URL}, transport)
	if off.Diagnostics != nil {
		t.Fatal("opt-out collected diagnostics")
	}
}

func TestHTTPDiagnosticsReusedConnectionAndTLS(t *testing.T) {
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) }))
	defer s.Close()
	transport := s.Client().Transport.(*http.Transport)
	defer transport.CloseIdleConnections()
	job := Job{URL: s.URL, CaptureDiagnostics: true}
	first := runCheck(job, transport)
	second := runCheck(job, transport)
	if !first.Status || !second.Status {
		t.Fatal("TLS request failed")
	}
	a, b := lastHTTPHop(first.Diagnostics), lastHTTPHop(second.Diagnostics)
	if *a.Reused || !*b.Reused {
		t.Fatalf("reuse: %+v %+v", a, b)
	}
	found := false
	for _, p := range a.Phases {
		if p.Name == "tls" && p.Complete {
			found = true
		}
	}
	if !found {
		t.Fatal("missing TLS phase")
	}
	for _, p := range b.Phases {
		if p.Name == "tls" || p.Name == "tcp" || p.Name == "dns" {
			t.Fatal("invented phases on reused connection")
		}
	}
}

func TestHTTPDiagnosticsResponseTimeout(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer s.Close()
	transport := checkTransport()
	defer transport.CloseIdleConnections()
	out := probeHTTP(Job{URL: s.URL, CaptureDiagnostics: true}, 30*time.Millisecond, transport)
	if out.up || out.diagnostic == nil {
		t.Fatal("missing failed trace")
	}
	h := out.diagnostic.Hops[0]
	if h.FailurePhase != "response_wait" {
		t.Fatalf("phase %q", h.FailurePhase)
	}
}

type traceRoundTripper func(*http.Request) (*http.Response, error)

func (f traceRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestHTTPDiagnosticsParallelDialAndLateHooks(t *testing.T) {
	var late func()
	tracer := &tracingTransport{base: traceRoundTripper(func(r *http.Request) (*http.Response, error) {
		trace := httptrace.ContextClientTrace(r.Context())
		trace.GetConn("example.test:443")
		trace.DNSStart(httptrace.DNSStartInfo{})
		trace.DNSDone(httptrace.DNSDoneInfo{})
		var wg sync.WaitGroup
		for _, addr := range []string{"[::1]:443", "127.0.0.1:443"} {
			wg.Add(1)
			go func(addr string) {
				defer wg.Done()
				trace.ConnectStart("tcp", addr)
				trace.ConnectDone("tcp", addr, errors.New("unreachable"))
			}(addr)
		}
		wg.Wait()
		late = func() { trace.TLSHandshakeStart(); trace.TLSHandshakeDone(tls.ConnectionState{}, errors.New("late")) }
		return nil, context.DeadlineExceeded
	})}
	r, _ := http.NewRequest("GET", "https://example.test", nil)
	_, _ = tracer.RoundTrip(r)
	d := tracer.snapshot(time.Now())
	before := len(d.Hops[0].Phases)
	done := make(chan struct{})
	go func() { late(); close(done) }()
	<-done
	if len(d.Hops[0].Phases) != before {
		t.Fatal("late hook mutated snapshot")
	}
	count := 0
	for _, p := range d.Hops[0].Phases {
		if p.Name == "tcp" {
			count++
		}
	}
	if count != 2 || d.Hops[0].FailurePhase != "tcp" {
		t.Fatalf("parallel dial evidence: %+v", d)
	}
}

func TestHTTPDiagnosticsDNSFailure(t *testing.T) {
	transport := checkTransport()
	defer transport.CloseIdleConnections()
	transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
		trace := httptrace.ContextClientTrace(ctx)
		trace.DNSStart(httptrace.DNSStartInfo{})
		err := errors.New("resolver unavailable")
		trace.DNSDone(httptrace.DNSDoneInfo{Err: err})
		return nil, err
	}
	out := probeHTTP(Job{URL: "http://example.test", CaptureDiagnostics: true}, time.Second, transport)
	if out.diagnostic.Hops[0].FailurePhase != "dns" {
		t.Fatalf("wrong DNS failure: %+v", out)
	}
}

func BenchmarkHTTPDiagnostics(b *testing.B) {
	for _, enabled := range []bool{false, true} {
		name := "off"
		if enabled {
			name = "on"
		}
		b.Run(name, func(b *testing.B) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) }))
			defer s.Close()
			transport := checkTransport()
			defer transport.CloseIdleConnections()
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				out := probeHTTP(Job{URL: s.URL, CaptureDiagnostics: enabled}, time.Second, transport)
				if !out.up {
					b.Fatal(out.err)
				}
			}
		})
	}
}

func TestHTTPDiagnosticsTraceBound(t *testing.T) {
	h := &hopTrace{start: time.Now(), active: make(map[string][]int)}
	for i := 0; i < 1000; i++ {
		h.begin("tcp", "tcp")
	}
	if len(h.hop.Phases) != maxTracePhases || !h.hop.Truncated {
		t.Fatal("unbounded trace")
	}
}

func TestHTTPDiagnosticsTLSFailureAndRedirectPolicy(t *testing.T) {
	tlsServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) }))
	defer tlsServer.Close()
	transport := checkTransport()
	defer transport.CloseIdleConnections()
	out := probeHTTP(Job{URL: tlsServer.URL, CaptureDiagnostics: true}, time.Second, transport)
	if out.up || out.diagnostic.FailurePhase != "tls" {
		t.Fatalf("TLS attribution: %+v", out.diagnostic)
	}
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/loop", http.StatusFound) }))
	defer redirect.Close()
	follow := false
	out = probeHTTP(Job{URL: redirect.URL, CaptureDiagnostics: true, RequestConfig: &db.RequestConfig{FollowRedirects: &follow}}, time.Second, transport)
	if !out.up || len(out.diagnostic.Hops) != 1 {
		t.Fatal("changed disabled redirect behavior")
	}
	out = probeHTTP(Job{URL: redirect.URL, CaptureDiagnostics: true}, time.Second, transport)
	if out.up || out.diagnostic.FailurePhase != "redirect" || len(out.diagnostic.Hops) != 10 {
		t.Fatalf("redirect limit: %+v", out.diagnostic)
	}
}

func TestHTTPDiagnosticsTransportRetriesAndEarlyResponse(t *testing.T) {
	left, right := net.Pipe()
	defer func() { _ = left.Close(); _ = right.Close() }()
	tracer := &tracingTransport{base: traceRoundTripper(func(r *http.Request) (*http.Response, error) {
		h := httptrace.ContextClientTrace(r.Context())
		h.GetConn("example.test:80")
		h.GotConn(httptrace.GotConnInfo{Conn: left, Reused: true})
		h.WroteRequest(httptrace.WroteRequestInfo{Err: errors.New("stale connection")})
		h.GetConn("example.test:80")
		h.GotConn(httptrace.GotConnInfo{Conn: left})
		// Early responses may precede completion of the request body.
		h.GotFirstResponseByte()
		h.WroteRequest(httptrace.WroteRequestInfo{})
		return &http.Response{StatusCode: 204, Body: http.NoBody}, nil
	})}
	r, _ := http.NewRequest("GET", "http://example.test", nil)
	_, err := tracer.RoundTrip(r)
	if err != nil {
		t.Fatal(err)
	}
	h := tracer.snapshot(time.Now()).Hops[0]
	seenFirst, seenSecond := false, false
	for _, p := range h.Phases {
		if p.Exchange == 1 {
			seenFirst = true
		}
		if p.Exchange == 2 {
			seenSecond = true
		}
		if p.Name == "response_wait" {
			t.Fatal("invented wait after an early response")
		}
	}
	if !seenFirst || !seenSecond || h.FailurePhase != "" {
		t.Fatalf("transport retry lost: %+v", h)
	}
}

func TestHTTPDiagnosticsMalformedRequest(t *testing.T) {
	out := probeHTTP(Job{URL: "://invalid", CaptureDiagnostics: true}, time.Second, checkTransport())
	if !out.fatal || out.diagnostic == nil || out.diagnostic.FailurePhase != "request" {
		t.Fatalf("missing request failure: %+v", out)
	}
	encoded, err := json.Marshal(out.diagnostic)
	if err != nil || !strings.Contains(string(encoded), `"hops":[]`) {
		t.Fatalf("missing empty hop list: %s %v", encoded, err)
	}
}
