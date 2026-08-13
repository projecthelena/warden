package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/projecthelena/warden/internal/db"
)

func TestValidateTarget(t *testing.T) {
	cases := []struct {
		monitorType string
		target      string
		wantErr     bool
	}{
		// http keeps the rules it already had
		{"", "https://example.com/health", false},
		{"http", "http://example.com", false},
		{"http", "https://example.com:8443/health?x=1", false},
		{"http", "ftp://example.com", true},
		{"http", "file:///etc/passwd", true},
		{"http", "example.com", true},

		{"tcp", "db.example.com:5432", false},
		{"tcp", "10.0.0.4:22", false},
		{"tcp", "[2001:db8::1]:5432", false},
		{"tcp", "example.com", true},
		{"tcp", "example.com:0", true},
		{"tcp", "example.com:70000", true},
		{"tcp", "http://example.com:80", true},

		{"ping", "example.com", false},
		{"ping", "192.168.1.1", false},
		{"ping", "2001:db8::1", false},
		{"ping", "example.com:80", true},
		{"ping", "http://example.com", true},
		{"ping", "example.com/health", true},

		{"dns", "example.com", false},
		{"dns", "sub.example.com.", false},
		{"dns", "https://example.com", true},
		// Looking up an IP literal returns it without querying anything, so a monitor
		// pointed at one could never fail. It has to be a name.
		{"dns", "1.1.1.1", true},
		{"dns", "2001:db8::1", true},

		// Docker's resolver serves compose service names containing underscores.
		{"tcp", "db_primary:5432", false},
		{"ping", "db_primary", false},
		{"dns", "db_primary.internal", false},

		{"gopher", "example.com", true},
	}

	for _, tc := range cases {
		err := validateTarget(tc.monitorType, tc.target)
		if tc.wantErr && err == nil {
			t.Errorf("validateTarget(%q, %q) = nil, want an error", tc.monitorType, tc.target)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("validateTarget(%q, %q) = %v, want nil", tc.monitorType, tc.target, err)
		}
	}
}

func TestValidateTargetRejectsOverlongTarget(t *testing.T) {
	long := "https://example.com/" + string(make([]byte, maxTargetLength))
	if err := validateTarget("http", long); err == nil {
		t.Fatal("expected an overlong target to be rejected")
	}
}

func TestValidateRequestConfigDNSOptions(t *testing.T) {
	if err := validateRequestConfig(&db.RequestConfig{DNSRecordType: "MX", DNSResolver: "1.1.1.1"}); err != nil {
		t.Fatalf("expected valid DNS options to pass, got %v", err)
	}
	if err := validateRequestConfig(&db.RequestConfig{DNSRecordType: "SRV"}); err == nil {
		t.Fatal("expected an unsupported record type to be rejected")
	}
	if err := validateRequestConfig(&db.RequestConfig{DNSResolver: "https://1.1.1.1"}); err == nil {
		t.Fatal("expected a resolver with a scheme to be rejected")
	}
	if err := validateRequestConfig(&db.RequestConfig{DNSResolver: "1.1.1.1:53"}); err != nil {
		t.Fatalf("expected a resolver with a port to pass, got %v", err)
	}
}

func TestCreateMonitorStoresType(t *testing.T) {
	crudH, _, _, _, s := setupTest(t)
	// setupTest seeds the default group.

	payload := map[string]any{
		"name":     "Router",
		"type":     "ping",
		"url":      "192.168.1.1",
		"groupId":  "g-default",
		"interval": 60,
	}
	body, _ := json.Marshal(payload)
	req := withAdminRole(httptest.NewRequest("POST", "/api/monitors", bytes.NewBuffer(body)))
	w := httptest.NewRecorder()
	crudH.CreateMonitor(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	monitors, _ := s.GetMonitors()
	if len(monitors) != 1 {
		t.Fatalf("expected 1 monitor, got %d", len(monitors))
	}
	if monitors[0].Type != db.MonitorTypePing {
		t.Errorf("expected type %q, got %q", db.MonitorTypePing, monitors[0].Type)
	}
}

func TestCreateMonitorRejectsTargetThatDoesNotMatchType(t *testing.T) {
	crudH, _, _, _, _ := setupTest(t)
	// setupTest seeds the default group.

	payload := map[string]any{
		"name":     "Bad Ping",
		"type":     "ping",
		"url":      "https://example.com",
		"groupId":  "g-default",
		"interval": 60,
	}
	body, _ := json.Marshal(payload)
	req := withAdminRole(httptest.NewRequest("POST", "/api/monitors", bytes.NewBuffer(body)))
	w := httptest.NewRecorder()
	crudH.CreateMonitor(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an URL used as a ping target, got %d", w.Code)
	}
}

func TestCreateMonitorRejectsUnknownType(t *testing.T) {
	crudH, _, _, _, _ := setupTest(t)
	// setupTest seeds the default group.

	payload := map[string]any{
		"name":     "Weird",
		"type":     "gopher",
		"url":      "example.com",
		"groupId":  "g-default",
		"interval": 60,
	}
	body, _ := json.Marshal(payload)
	req := withAdminRole(httptest.NewRequest("POST", "/api/monitors", bytes.NewBuffer(body)))
	w := httptest.NewRecorder()
	crudH.CreateMonitor(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an unknown type, got %d", w.Code)
	}
}

// An API client that only renames a monitor must not turn it back into an HTTP check.
func TestUpdateMonitorWithoutTypeKeepsStoredType(t *testing.T) {
	crudH, _, _, _, s := setupTest(t)

	seed := db.Monitor{ID: "m-ping", Type: db.MonitorTypePing, GroupID: "g-default", Name: "Router", URL: "192.168.1.1", Interval: 60}
	if err := s.CreateMonitor(seed); err != nil {
		t.Fatalf("failed to seed monitor: %v", err)
	}

	payload := map[string]any{"name": "Router (LAN)", "url": "192.168.1.1", "interval": 60}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("PUT", "/api/monitors/m-ping", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Use(adminRoleMiddleware)
	r.Put("/api/monitors/{id}", crudH.UpdateMonitor)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	monitors, _ := s.GetMonitors()
	if monitors[0].Type != db.MonitorTypePing {
		t.Errorf("expected the stored type to survive the update, got %q", monitors[0].Type)
	}
}

func TestUpdateMonitorCanChangeType(t *testing.T) {
	crudH, _, _, _, s := setupTest(t)

	seed := db.Monitor{ID: "m-http", GroupID: "g-default", Name: "API", URL: "https://example.com", Interval: 60}
	if err := s.CreateMonitor(seed); err != nil {
		t.Fatalf("failed to seed monitor: %v", err)
	}

	payload := map[string]any{"name": "API", "type": "tcp", "url": "example.com:443", "interval": 60}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("PUT", "/api/monitors/m-http", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Use(adminRoleMiddleware)
	r.Put("/api/monitors/{id}", crudH.UpdateMonitor)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	monitors, _ := s.GetMonitors()
	if monitors[0].Type != db.MonitorTypeTCP {
		t.Errorf("expected type tcp, got %q", monitors[0].Type)
	}
	if monitors[0].URL != "example.com:443" {
		t.Errorf("expected the target to change with the type, got %q", monitors[0].URL)
	}
}
