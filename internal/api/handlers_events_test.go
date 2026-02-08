package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/projecthelena/warden/internal/db"
	"github.com/projecthelena/warden/internal/uptime"
)

func TestGetSystemEvents(t *testing.T) {
	s, _ := db.NewStore(db.NewTestConfig())
	m := uptime.NewManager(s)
	h := NewEventHandler(s, m)

	req := httptest.NewRequest("GET", "/api/events", nil)
	w := httptest.NewRecorder()

	h.GetSystemEvents(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

func TestGetSystemEvents_EmptyResponse(t *testing.T) {
	s, _ := db.NewStore(db.NewTestConfig())
	m := uptime.NewManager(s)
	h := NewEventHandler(s, m)

	req := httptest.NewRequest("GET", "/api/events", nil)
	w := httptest.NewRecorder()

	h.GetSystemEvents(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Verify all arrays exist and are empty
	active, ok := response["active"].([]interface{})
	if !ok {
		t.Fatal("Expected 'active' to be an array")
	}
	if len(active) != 0 {
		t.Errorf("Expected empty active array, got %d items", len(active))
	}

	history, ok := response["history"].([]interface{})
	if !ok {
		t.Fatal("Expected 'history' to be an array")
	}
	if len(history) != 0 {
		t.Errorf("Expected empty history array, got %d items", len(history))
	}

	sslWarnings, ok := response["sslWarnings"].([]interface{})
	if !ok {
		t.Fatal("Expected 'sslWarnings' to be an array")
	}
	if len(sslWarnings) != 0 {
		t.Errorf("Expected empty sslWarnings array, got %d items", len(sslWarnings))
	}
}

func TestGetSystemEvents_WithSSLWarnings(t *testing.T) {
	s, _ := db.NewStore(db.NewTestConfig())
	m := uptime.NewManager(s)
	h := NewEventHandler(s, m)

	// Setup test data
	_ = s.CreateGroup(db.Group{ID: "g1", Name: "Production"})
	_ = s.CreateMonitor(db.Monitor{ID: "m1", GroupID: "g1", Name: "API Server", URL: "https://api.example.com", Interval: 60})
	_ = s.CreateEvent("m1", "ssl_expiring", "SSL certificate expires in 14 days (2025-02-15)")

	req := httptest.NewRequest("GET", "/api/events", nil)
	w := httptest.NewRecorder()

	h.GetSystemEvents(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	sslWarnings, ok := response["sslWarnings"].([]interface{})
	if !ok {
		t.Fatal("Expected 'sslWarnings' to be an array")
	}
	if len(sslWarnings) != 1 {
		t.Fatalf("Expected 1 SSL warning, got %d", len(sslWarnings))
	}

	warning := sslWarnings[0].(map[string]interface{})
	if warning["monitorId"] != "m1" {
		t.Errorf("Expected monitorId 'm1', got %v", warning["monitorId"])
	}
	if warning["monitorName"] != "API Server" {
		t.Errorf("Expected monitorName 'API Server', got %v", warning["monitorName"])
	}
	if warning["groupName"] != "Production" {
		t.Errorf("Expected groupName 'Production', got %v", warning["groupName"])
	}
	if warning["groupId"] != "g1" {
		t.Errorf("Expected groupId 'g1', got %v", warning["groupId"])
	}
	if warning["type"] != "ssl_expiring" {
		t.Errorf("Expected type 'ssl_expiring', got %v", warning["type"])
	}
	if warning["message"] != "SSL certificate expires in 14 days (2025-02-15)" {
		t.Errorf("Unexpected message: %v", warning["message"])
	}
}

func TestGetSystemEvents_MixedContent(t *testing.T) {
	s, _ := db.NewStore(db.NewTestConfig())
	m := uptime.NewManager(s)
	h := NewEventHandler(s, m)

	// Setup: group, monitors, outages, and SSL events
	_ = s.CreateGroup(db.Group{ID: "g1", Name: "Production"})
	_ = s.CreateMonitor(db.Monitor{ID: "m1", GroupID: "g1", Name: "API", Interval: 60})
	_ = s.CreateMonitor(db.Monitor{ID: "m2", GroupID: "g1", Name: "Web", Interval: 60})

	// Active outage
	_ = s.CreateOutage("m1", "down", "Connection refused")

	// SSL warning
	_ = s.CreateEvent("m2", "ssl_expiring", "SSL certificate expires in 7 days")

	req := httptest.NewRequest("GET", "/api/events", nil)
	w := httptest.NewRecorder()

	h.GetSystemEvents(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Verify active outages
	active := response["active"].([]interface{})
	if len(active) != 1 {
		t.Errorf("Expected 1 active outage, got %d", len(active))
	}

	// Verify SSL warnings
	sslWarnings := response["sslWarnings"].([]interface{})
	if len(sslWarnings) != 1 {
		t.Errorf("Expected 1 SSL warning, got %d", len(sslWarnings))
	}
}

func TestGetSystemEvents_MultipleSSLWarnings(t *testing.T) {
	s, _ := db.NewStore(db.NewTestConfig())
	m := uptime.NewManager(s)
	h := NewEventHandler(s, m)

	// Setup multiple monitors with SSL warnings
	_ = s.CreateGroup(db.Group{ID: "g1", Name: "Production"})
	_ = s.CreateGroup(db.Group{ID: "g2", Name: "Staging"})
	_ = s.CreateMonitor(db.Monitor{ID: "m1", GroupID: "g1", Name: "API", Interval: 60})
	_ = s.CreateMonitor(db.Monitor{ID: "m2", GroupID: "g1", Name: "Web", Interval: 60})
	_ = s.CreateMonitor(db.Monitor{ID: "m3", GroupID: "g2", Name: "Staging API", Interval: 60})

	_ = s.CreateEvent("m1", "ssl_expiring", "Expires in 30 days")
	_ = s.CreateEvent("m2", "ssl_expiring", "Expires in 14 days")
	_ = s.CreateEvent("m3", "ssl_expiring", "Expires in 1 day")

	req := httptest.NewRequest("GET", "/api/events", nil)
	w := httptest.NewRecorder()

	h.GetSystemEvents(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	sslWarnings := response["sslWarnings"].([]interface{})
	if len(sslWarnings) != 3 {
		t.Fatalf("Expected 3 SSL warnings, got %d", len(sslWarnings))
	}

	// Verify each warning has required fields
	for i, w := range sslWarnings {
		warning := w.(map[string]interface{})
		requiredFields := []string{"id", "monitorId", "monitorName", "groupName", "groupId", "type", "message", "timestamp"}
		for _, field := range requiredFields {
			if _, ok := warning[field]; !ok {
				t.Errorf("Warning %d missing field: %s", i, field)
			}
		}
	}
}
