package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireSameOrigin(t *testing.T) {
	handler := requireSameOrigin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	cases := []struct {
		name   string
		origin string
		want   int
	}{
		// MCP clients are not browsers, so the usual case carries no Origin at all.
		{"no origin", "", http.StatusOK},
		{"same origin", "https://warden.example.com", http.StatusOK},
		{"same origin, different scheme", "http://warden.example.com", http.StatusOK},
		{"another site", "https://evil.example.com", http.StatusForbidden},
		{"lookalike suffix", "https://notwarden.example.com", http.StatusForbidden},
		{"unparseable", "://nonsense", http.StatusForbidden},
	}

	for _, tc := range cases {
		req := httptest.NewRequest("POST", "https://warden.example.com/api/mcp", nil)
		req.Host = "warden.example.com"
		if tc.origin != "" {
			req.Header.Set("Origin", tc.origin)
		}

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != tc.want {
			t.Errorf("%s: expected %d, got %d", tc.name, tc.want, w.Code)
		}
	}
}
