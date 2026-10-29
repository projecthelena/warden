package api

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/projecthelena/warden/internal/config"
	"github.com/projecthelena/warden/internal/db"
	"github.com/projecthelena/warden/internal/uptime"
)

func TestRouterCompressesPublicStatusJSON(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	if err := store.UpsertStatusPage("compressed", "Compressed status", nil, true, true); err != nil {
		t.Fatalf("seed status page: %v", err)
	}

	router := NewRouter(uptime.NewManager(store), store, &config.Config{})
	request := httptest.NewRequest(http.MethodGet, "/api/s/compressed", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("expected gzip content encoding, got %q", got)
	}
	if got := response.Header().Get("Vary"); !strings.Contains(got, "Accept-Encoding") {
		t.Fatalf("expected Vary: Accept-Encoding, got %q", got)
	}
	reader, err := gzip.NewReader(response.Body)
	if err != nil {
		t.Fatalf("open gzip response: %v", err)
	}
	decoded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read gzip response: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(decoded, &payload); err != nil {
		t.Fatalf("compressed body is not valid JSON: %v", err)
	}

	identityRequest := httptest.NewRequest(http.MethodGet, "/api/s/compressed", nil)
	identityResponse := httptest.NewRecorder()
	router.ServeHTTP(identityResponse, identityRequest)
	if got := identityResponse.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("identity request unexpectedly encoded as %q", got)
	}
	if got := identityResponse.Header().Values("Vary"); len(got) != 1 || got[0] != "Accept-Encoding" {
		t.Fatalf("identity response must carry Vary: Accept-Encoding once, got %q", got)
	}
	if !json.Valid(identityResponse.Body.Bytes()) {
		t.Fatalf("identity body is not plain JSON")
	}
	if got := response.Header().Values("Vary"); len(got) != 1 {
		t.Fatalf("gzip response must carry Vary: Accept-Encoding once, got %q", got)
	}
}

func TestCompressResponsesLeavesStreamsAndEncodedBodiesAlone(t *testing.T) {
	cases := map[string]http.HandlerFunc{
		"event stream": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "data: hello\n\n")
			w.(http.Flusher).Flush()
		},
		"precompressed": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/javascript")
			w.Header().Set("Content-Encoding", "br")
			_, _ = io.WriteString(w, "already-encoded")
		},
	}
	for name, handler := range cases {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.Header.Set("Accept-Encoding", "gzip")
			response := httptest.NewRecorder()
			compressResponses(handler).ServeHTTP(response, request)

			if got := response.Header().Get("Content-Encoding"); got == "gzip" {
				t.Fatalf("response was gzipped again")
			}
			if body := response.Body.String(); body != "data: hello\n\n" && body != "already-encoded" {
				t.Fatalf("body was rewritten: %q", body)
			}
			if name == "event stream" && !response.Flushed {
				t.Fatalf("event stream flush did not reach the client")
			}
		})
	}
}

// Every response through the compressor carries Vary: Accept-Encoding exactly once,
// whether it was encoded, refused gzip with q=0, or has a type Compress skips.
func TestCompressResponsesVaryOnce(t *testing.T) {
	cases := []struct {
		name, acceptEncoding, contentType, wantEncoding string
	}{
		{"gzip json", "gzip", "application/json", "gzip"},
		{"no accept-encoding", "", "application/json", ""},
		// Compress ignores q-values and may still gzip; only Vary is asserted here.
		{"gzip refused with q=0", "gzip;q=0", "application/json", "any"},
		{"png", "gzip", "image/png", ""},
		{"plain text", "gzip", "text/plain", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler := compressResponses(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				_, _ = io.WriteString(w, strings.Repeat("x", 2048))
			}))
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.acceptEncoding != "" {
				request.Header.Set("Accept-Encoding", tc.acceptEncoding)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if got := response.Header().Get("Content-Encoding"); tc.wantEncoding != "any" && got != tc.wantEncoding {
				t.Fatalf("Content-Encoding = %q, want %q", got, tc.wantEncoding)
			}
			if got := response.Header().Values("Vary"); len(got) != 1 || got[0] != "Accept-Encoding" {
				t.Fatalf("Vary = %q, want Accept-Encoding once", got)
			}
		})
	}
}
