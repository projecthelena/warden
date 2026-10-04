package uptime

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptrace"
	"sync"
	"time"

	"github.com/projecthelena/warden/internal/db"
)

const maxTracePhases = 64

// A transport wrapper gives each redirect its own trace. Hooks may run concurrently
// and even after RoundTrip returns; snapshots never share mutable hook state.
type tracingTransport struct {
	base      http.RoundTripper
	mu        sync.Mutex
	hops      []db.HTTPHop
	truncated bool
}

type hopTrace struct {
	mu        sync.Mutex
	start     time.Time
	hop       db.HTTPHop
	firstByte bool
	exchange  int
	active    map[string][]int
}

func elapsedMS(start time.Time) float64 {
	return float64(time.Since(start)) / float64(time.Millisecond)
}

func (t *hopTrace) begin(name, key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.beginLocked(name, key)
}

func (t *hopTrace) beginLocked(name, key string) {
	if len(t.hop.Phases) >= maxTracePhases {
		t.hop.Truncated = true
		return
	}
	i := len(t.hop.Phases)
	t.hop.Phases = append(t.hop.Phases, db.HTTPPhase{Exchange: t.exchange, Name: name, StartMS: elapsedMS(t.start)})
	t.active[key] = append(t.active[key], i)
}

func (t *hopTrace) end(name string, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.endLocked(name, err)
}

func (t *hopTrace) endLocked(key string, err error) {
	indices := t.active[key]
	if len(indices) == 0 {
		return
	}
	i := indices[0]
	t.active[key] = indices[1:]
	p := &t.hop.Phases[i]
	p.DurationMS = elapsedMS(t.start) - p.StartMS
	p.Complete = true
	p.Failed = err != nil
}

func (t *tracingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	h := &hopTrace{start: time.Now(), hop: db.HTTPHop{Host: req.URL.Hostname(), Phases: []db.HTTPPhase{}}, active: make(map[string][]int)}
	trace := &httptrace.ClientTrace{
		GetConn: func(string) {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.exchange++
			h.firstByte = false
			h.beginLocked("connection_acquire", "acquire")
		},
		DNSStart:          func(httptrace.DNSStartInfo) { h.begin("dns", "dns") },
		DNSDone:           func(i httptrace.DNSDoneInfo) { h.end("dns", i.Err) },
		ConnectStart:      func(network, addr string) { h.begin("tcp", network+addr) },
		ConnectDone:       func(network, addr string, err error) { h.end(network+addr, err) },
		TLSHandshakeStart: func() { h.begin("tls", "tls") },
		TLSHandshakeDone:  func(_ tls.ConnectionState, err error) { h.end("tls", err) },
		GotConn: func(i httptrace.GotConnInfo) {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.endLocked("acquire", nil)
			reused := i.Reused
			h.hop.Reused = &reused
			ip, _, err := net.SplitHostPort(i.Conn.RemoteAddr().String())
			if err == nil {
				h.hop.RemoteIP = ip
			}
			h.beginLocked("send", "send")
		},
		WroteRequest: func(i httptrace.WroteRequestInfo) {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.endLocked("send", i.Err)
			if i.Err == nil && !h.firstByte {
				h.beginLocked("response_wait", "wait")
			}
		},
		GotFirstResponseByte: func() { h.mu.Lock(); defer h.mu.Unlock(); h.firstByte = true; h.endLocked("wait", nil) },
	}
	resp, err := t.base.RoundTrip(req.WithContext(httptrace.WithClientTrace(req.Context(), trace)))
	h.mu.Lock()
	h.hop.TotalMS = elapsedMS(h.start)
	if resp != nil {
		h.hop.StatusCode = resp.StatusCode
	}
	if err != nil {
		h.hop.FailurePhase = "unknown"
		// Prefer a specific phase over the encompassing connection acquisition.
		for _, p := range h.hop.Phases {
			if p.Exchange == h.exchange && (p.Failed || !p.Complete) {
				h.hop.FailurePhase = p.Name
			}
		}
	}
	for i := range h.hop.Phases {
		p := &h.hop.Phases[i]
		if !p.Complete {
			p.DurationMS = h.hop.TotalMS - p.StartMS
		}
	}
	snapshot := h.hop
	snapshot.Phases = append([]db.HTTPPhase{}, h.hop.Phases...)
	h.mu.Unlock()
	t.mu.Lock()
	if len(t.hops) < 11 {
		t.hops = append(t.hops, snapshot)
	} else {
		t.truncated = true
	}
	t.mu.Unlock()
	return resp, err
}

func (t *tracingTransport) snapshot(start time.Time) db.HTTPAttempt {
	t.mu.Lock()
	defer t.mu.Unlock()
	return db.HTTPAttempt{TotalMS: elapsedMS(start), Hops: append([]db.HTTPHop{}, t.hops...), Truncated: t.truncated}
}
