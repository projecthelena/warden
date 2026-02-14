package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/projecthelena/warden/internal/config"
	"github.com/projecthelena/warden/internal/db"
	"github.com/projecthelena/warden/internal/uptime"
)

func TestHealthz(t *testing.T) {
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()

	Healthz(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("expected status ok, got %v", resp["status"])
	}
}

func TestReadyz(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	handler := Readyz(store)

	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("expected status ok, got %v", resp["status"])
	}
}

func TestReadyz_DBDown(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	// Close the DB to simulate it being unavailable
	store.Close()

	handler := Readyz(store)

	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["status"] != "unavailable" {
		t.Errorf("expected status unavailable, got %v", resp["status"])
	}
}

// TestHealthProbes_Integration verifies /healthz and /readyz work through
// the full router (middleware chain, no auth required).
func TestHealthProbes_Integration(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "health-test.db")
	store, err := db.NewStore(db.NewTestConfigWithPath(dbPath))
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	manager := uptime.NewManager(store)
	cfg := config.Default()
	router := NewRouter(manager, store, &cfg)

	ts := httptest.NewServer(router)
	defer ts.Close()

	client := ts.Client()

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantBody   string
	}{
		{"healthz returns 200", "/healthz", http.StatusOK, "ok"},
		{"readyz returns 200", "/readyz", http.StatusOK, "ok"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := client.Get(ts.URL + tt.path)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("expected %d, got %d", tt.wantStatus, resp.StatusCode)
			}

			var body map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if body["status"] != tt.wantBody {
				t.Errorf("expected status %q, got %v", tt.wantBody, body["status"])
			}

			// Verify JSON content type
			ct := resp.Header.Get("Content-Type")
			if ct != "application/json" {
				t.Errorf("expected Content-Type application/json, got %q", ct)
			}
		})
	}
}
