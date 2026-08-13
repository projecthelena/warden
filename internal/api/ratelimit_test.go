package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRealIPFromProxy pins the behaviour that made chi's middleware.RealIP unsafe:
// only the rightmost X-Forwarded-For entry (the one our own proxy appended) may be
// trusted. Anything a caller puts to the left of it must be ignored, otherwise the
// login rate limiter and the audit log can both be defeated with a header.
func TestRealIPFromProxy(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		wantIP     string
	}{
		{
			name:       "no headers keeps remote addr",
			remoteAddr: "192.0.2.10:5555",
			wantIP:     "192.0.2.10",
		},
		{
			name:       "single hop uses forwarded client",
			remoteAddr: "10.0.0.1:5555",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.7"},
			wantIP:     "203.0.113.7",
		},
		{
			name:       "spoofed prefix is ignored, rightmost wins",
			remoteAddr: "10.0.0.1:5555",
			headers:    map[string]string{"X-Forwarded-For": "1.1.1.1, 2.2.2.2, 203.0.113.7"},
			wantIP:     "203.0.113.7",
		},
		{
			name:       "garbage forwarded value falls back to remote addr",
			remoteAddr: "10.0.0.1:5555",
			headers:    map[string]string{"X-Forwarded-For": "not-an-ip"},
			wantIP:     "10.0.0.1",
		},
		{
			name:       "X-Real-IP alone is not trusted",
			remoteAddr: "10.0.0.1:5555",
			headers:    map[string]string{"X-Real-IP": "203.0.113.99"},
			wantIP:     "10.0.0.1",
		},
		{
			name:       "True-Client-IP alone is not trusted",
			remoteAddr: "10.0.0.1:5555",
			headers:    map[string]string{"True-Client-IP": "203.0.113.99"},
			wantIP:     "10.0.0.1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			h := RealIPFromProxy(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got = extractIP(r)
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tc.remoteAddr
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			h.ServeHTTP(httptest.NewRecorder(), req)

			if got != tc.wantIP {
				t.Errorf("extractIP = %q, want %q", got, tc.wantIP)
			}
		})
	}
}

// TestRealIPFromProxyRateLimitNotBypassable is the end-to-end version: a caller
// rotating X-Forwarded-For must still land in the same rate limit bucket.
func TestRealIPFromProxyRateLimitNotBypassable(t *testing.T) {
	limiter := NewIPRateLimiter(1, 1)
	h := RealIPFromProxy(RateLimitMiddleware(limiter)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	call := func(xff string) int {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.1:5555"
		req.Header.Set("X-Forwarded-For", xff+", 203.0.113.7")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := call("1.1.1.1"); code != http.StatusOK {
		t.Fatalf("first request = %d, want 200", code)
	}
	// Different spoofed prefix, same real client: must still be limited.
	if code := call("2.2.2.2"); code != http.StatusTooManyRequests {
		t.Errorf("second request = %d, want 429 (spoofed prefix bypassed the limiter)", code)
	}
}
