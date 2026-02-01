package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/clusteruptime/clusteruptime/internal/config"
	"github.com/clusteruptime/clusteruptime/internal/db"
	"github.com/clusteruptime/clusteruptime/internal/uptime"
)

func TestAuthEnforcement(t *testing.T) {
	// Setup
	dbPath := filepath.Join(t.TempDir(), "auth_test.db")
	store, _ := db.NewStore(db.NewTestConfigWithPath(dbPath))
	manager := uptime.NewManager(store)
	cfg := config.Default()
	router := NewRouter(manager, store, &cfg)

	ts := httptest.NewServer(router)
	defer ts.Close()

	client := ts.Client() // No cookie jar, no auth headers

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"Overview", "GET", "/api/overview"},
		{"Me", "GET", "/api/auth/me"},
		{"Update User", "PATCH", "/api/auth/me"},
		{"Create Group", "POST", "/api/groups"},
		{"Update Group", "PUT", "/api/groups/g-test"},
		{"Delete Group", "DELETE", "/api/groups/g-test"},
		{"Get Uptime", "GET", "/api/uptime"},
		{"Create Monitor", "POST", "/api/monitors"},
		{"Update Monitor", "PUT", "/api/monitors/m-test"},
		{"Delete Monitor", "DELETE", "/api/monitors/m-test"},
		{"Monitor Uptime", "GET", "/api/monitors/m-test/uptime"},
		{"Monitor Latency", "GET", "/api/monitors/m-test/latency"},
		{"Get Incidents", "GET", "/api/incidents"},
		{"Create Incident", "POST", "/api/incidents"},
		{"Get Maintenance", "GET", "/api/maintenance"},
		{"Create Maintenance", "POST", "/api/maintenance"},
		{"Update Maintenance", "PUT", "/api/maintenance/1"},
		{"Delete Maintenance", "DELETE", "/api/maintenance/1"},
		{"Get Settings", "GET", "/api/settings"},
		{"Update Settings", "PATCH", "/api/settings"},
		{"List API Keys", "GET", "/api/api-keys"},
		{"Create API Key", "POST", "/api/api-keys"},
		{"Delete API Key", "DELETE", "/api/api-keys/1"},
		{"Get Stats", "GET", "/api/stats"},
		{"List Notification Channels", "GET", "/api/notifications/channels"},
		{"Create Notification Channel", "POST", "/api/notifications/channels"},
		{"Delete Notification Channel", "DELETE", "/api/notifications/channels/1"},
		{"Get Events", "GET", "/api/events"},
		{"List Status Pages", "GET", "/api/status-pages"},
		{"Toggle Status Page", "PATCH", "/api/status-pages/slug"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(tc.method, ts.URL+tc.path, nil)
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Failed to make request: %v", err)
			}
			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("Expected 401 Unauthorized for %s %s, got %d", tc.method, tc.path, resp.StatusCode)
			}
		})
	}
}
