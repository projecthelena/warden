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
}
