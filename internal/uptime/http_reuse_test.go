package uptime

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/projecthelena/warden/internal/db"
)

func TestHTTPChecksReuseConnections(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		tls, chunked, redirect bool
		status                 int
		accepted               string
	}{
		{name: "healthy", status: 200},
		{name: "chunked", status: 200, chunked: true},
		{name: "tls", status: 200, tls: true},
		{name: "redirect", status: 200, redirect: true},
		{name: "accepted error status", status: 503, accepted: "503"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var connections atomic.Int64
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.redirect && r.URL.Path != "/final" {
					http.Redirect(w, r, "/final", http.StatusFound)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				if tc.chunked {
					w.(http.Flusher).Flush()
				}
				_, _ = fmt.Fprintln(w, `{"ok":true,"service":"faketarget"}`)
			}))
			server.Config.ConnState = func(_ net.Conn, s http.ConnState) {
				if s == http.StateNew {
					connections.Add(1)
				}
			}
			if tc.tls {
				server.StartTLS()
			} else {
				server.Start()
			}
			defer server.Close()
			transport := checkTransport()
			if tc.tls {
				transport.TLSClientConfig = server.Client().Transport.(*http.Transport).TLSClientConfig
			}
			defer transport.CloseIdleConnections()
			job := Job{URL: server.URL, CaptureDiagnostics: true, RequestConfig: &db.RequestConfig{AcceptedStatusCodes: tc.accepted}}
			for i := 0; i < 100; i++ {
				result := probeHTTP(job, time.Second, transport)
				if !result.up || result.statusCode != tc.status {
					t.Fatalf("check %d: %+v", i, result)
				}
				if i > 0 && (result.diagnostic.Hops[0].Reused == nil || !*result.diagnostic.Hops[0].Reused) {
					t.Fatal("trace did not report reuse")
				}
				if tc.tls && result.certExpiry == nil {
					t.Fatal("missing TLS expiry")
				}
			}
			if connections.Load() != 1 {
				t.Fatalf("100 checks opened %d TCP connections; want 1", connections.Load())
			}
		})
	}
}

func TestHTTPBodyCleanupPreservesStatusAndLatency(t *testing.T) {
	for _, tc := range []struct {
		name    string
		length  int
		timeout time.Duration
	}{
		{"stalled chunked", -1, time.Second},
		{"stalled small", 100, time.Second},
		{"short monitor timeout", -1, 25 * time.Millisecond},
		{"large body", maxResponseDrainBytes + 1, time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.length >= 0 {
					w.Header().Set("Content-Length", strconv.Itoa(tc.length))
				}
				w.WriteHeader(http.StatusOK)
				w.(http.Flusher).Flush()
				<-r.Context().Done()
			}))
			defer server.Close()
			transport := checkTransport()
			defer transport.CloseIdleConnections()
			started := time.Now()
			result := probeHTTP(Job{URL: server.URL, CaptureDiagnostics: true}, tc.timeout, transport)
			elapsed := time.Since(started)
			if !result.up || result.statusCode != 200 || result.err != "" {
				t.Fatalf("cleanup changed status: %+v", result)
			}
			if elapsed > 500*time.Millisecond {
				t.Fatalf("cleanup took %s", elapsed)
			}
			if tc.name == "stalled small" || tc.name == "stalled chunked" {
				if elapsed-result.latency < responseDrainTimeout/2 {
					t.Fatalf("cleanup included in latency: total=%s latency=%s", elapsed, result.latency)
				}
			}
			if result.diagnostic.Status != "up" || result.diagnostic.FailurePhase != "" {
				t.Fatalf("cleanup changed diagnostics: %+v", result.diagnostic)
			}
		})
	}
}

func TestHTTPTruncatedBodyKeepsStatusOnlyResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		_, _ = io.WriteString(w, "short")
	}))
	defer server.Close()
	transport := checkTransport()
	defer transport.CloseIdleConnections()
	result := runCheck(Job{URL: server.URL, CaptureDiagnostics: true}, transport)
	if !result.Status || result.StatusCode != 200 || result.Error != "" {
		t.Fatalf("truncated body changed status: %+v", result)
	}
}

type drainReader struct{ read int }

func (r *drainReader) Read(p []byte) (int, error) { r.read += len(p); return len(p), nil }

func TestHTTPResponseDrainByteLimit(t *testing.T) {
	for _, length := range []int64{-1, maxResponseDrainBytes + 1} {
		r := &drainReader{}
		drainResponseBody(&http.Response{Body: io.NopCloser(r), ContentLength: length}, func() {})
		want := maxResponseDrainBytes + 1
		if length > maxResponseDrainBytes {
			want = 0
		}
		if r.read != want {
			t.Fatalf("length=%d read=%d want=%d", length, r.read, want)
		}
	}
}

func TestHTTPFailedResponseStillCapturesBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, "upstream unavailable")
	}))
	defer server.Close()
	transport := checkTransport()
	defer transport.CloseIdleConnections()
	result := probeHTTP(Job{URL: server.URL, CaptureDiagnostics: true}, time.Second, transport)
	if result.up || result.statusCode != 503 || !strings.Contains(result.respBody, "upstream unavailable") {
		t.Fatalf("failed response changed: %+v", result)
	}
}

func TestHTTPReportedLatencyExcludesBodyCleanup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()
	transport := checkTransport()
	defer transport.CloseIdleConnections()
	started := time.Now()
	result := runCheck(Job{URL: server.URL, CaptureDiagnostics: true}, transport)
	elapsed := time.Since(started)
	if !result.Status {
		t.Fatalf("cleanup changed status: %+v", result)
	}
	if elapsed-time.Duration(result.Latency)*time.Millisecond < responseDrainTimeout/2 {
		t.Fatalf("reported latency includes cleanup: elapsed=%s latency=%dms", elapsed, result.Latency)
	}
}
