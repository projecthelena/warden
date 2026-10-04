package uptime

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/projecthelena/warden/internal/db"
)

// SecondaryProbe is optional corroboration, never an input to availability or
// incident transitions. A burst can occupy at most two check workers for 3 seconds.
type SecondaryProbe struct {
	endpoint string
	token    string
	client   *http.Client
	slots    chan struct{}
	mu       sync.Mutex
	last     time.Time
}

func NewSecondaryProbe(endpoint, token string) (*SecondaryProbe, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("invalid secondary probe endpoint")
	}
	ip := net.ParseIP(u.Hostname())
	loopbackHTTP := u.Scheme == "http" && ip != nil && ip.IsLoopback()
	if u.Scheme != "https" && !loopbackHTTP {
		return nil, errors.New("secondary probe requires HTTPS or an HTTP loopback tunnel")
	}
	if len(token) < 32 {
		return nil, errors.New("secondary probe token must contain at least 32 characters")
	}
	return &SecondaryProbe{endpoint: strings.TrimRight(endpoint, "/"), token: token, slots: make(chan struct{}, 2), client: &http.Client{Timeout: 3 * time.Second, Transport: checkTransport(), CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

func (p *SecondaryProbe) Observe(id string) *db.ExternalObservation {
	unavailable := &db.ExternalObservation{Outcome: "unavailable"}
	p.mu.Lock()
	if time.Since(p.last) < time.Second {
		p.mu.Unlock()
		return &db.ExternalObservation{Outcome: "rate_limited"}
	}
	p.last = time.Now()
	p.mu.Unlock()
	select {
	case p.slots <- struct{}{}:
		defer func() { <-p.slots }()
	default:
		return &db.ExternalObservation{Outcome: "rate_limited"}
	}
	req, err := http.NewRequest(http.MethodGet, p.endpoint+"/v1/check/"+url.PathEscape(id), nil)
	if err != nil {
		return unavailable
	}
	req.Header.Set("Authorization", "Bearer "+p.token)
	resp, err := p.client.Do(req)
	if err != nil {
		return unavailable
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return unavailable
	}
	var observation db.ExternalObservation
	if json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&observation) != nil {
		return unavailable
	}
	at, err := time.Parse(time.RFC3339Nano, observation.ObservedAt)
	if err != nil || time.Since(at) > 30*time.Second || time.Until(at) > 30*time.Second {
		return unavailable
	}
	if observation.Outcome != "response" && observation.Outcome != "failed" {
		return unavailable
	}
	if observation.Outcome == "response" && (observation.StatusCode < 100 || observation.StatusCode > 599) {
		return unavailable
	}
	switch observation.FailurePhase {
	case "", "dns", "tcp", "tls", "send", "response_wait", "connection_acquire", "unknown":
	default:
		return unavailable
	}
	return &observation
}

// NewProbeHandler exposes only operator-registered IDs. The caller cannot supply
// URLs or request headers. The returned observation contains no credentials/body.
func NewProbeHandler(token string, targets map[string]string) (http.Handler, error) {
	if len(token) < 32 {
		return nil, errors.New("probe token must contain at least 32 characters")
	}
	registered := make(map[string]string, len(targets))
	for id, target := range targets {
		u, err := url.Parse(target)
		if id == "" || strings.ContainsAny(id, "/\\") || err != nil || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return nil, errors.New("invalid probe target")
		}
		registered[id] = target
	}
	transport := checkTransport()
	slots := make(chan struct{}, 2)
	expected := sha256.Sum256([]byte("Bearer " + token))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		given := sha256.Sum256([]byte(r.Header.Get("Authorization")))
		if subtle.ConstantTimeCompare(given[:], expected[:]) != 1 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id, ok := strings.CutPrefix(r.URL.Path, "/v1/check/")
		target, registeredOK := registered[id]
		if !ok || !registeredOK {
			http.NotFound(w, r)
			return
		}
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			http.Error(w, "Busy", http.StatusTooManyRequests)
			return
		}
		out := probeHTTP(Job{URL: target, CaptureDiagnostics: true}, 2*time.Second, transport)
		observation := db.ExternalObservation{ObservedAt: time.Now().UTC().Format(time.RFC3339Nano), Outcome: "failed", StatusCode: out.statusCode}
		if out.statusCode > 0 {
			observation.Outcome = "response"
		}
		if out.diagnostic != nil && len(out.diagnostic.Hops) > 0 {
			observation.FailurePhase = out.diagnostic.Hops[len(out.diagnostic.Hops)-1].FailurePhase
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(observation)
	}), nil
}
