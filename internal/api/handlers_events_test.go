package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

// TestGetSystemEvents_DateFilter exercises the ?date=YYYY-MM-DD path that powers the Slack
// digest deep-link into /incidents?date=... The filter should narrow History to outages that
// started on that UTC day.
func TestGetSystemEvents_DateFilter(t *testing.T) {
	s, _ := db.NewStore(db.NewTestConfig())
	m := uptime.NewManager(s)
	h := NewEventHandler(s, m)

	_ = s.CreateGroup(db.Group{ID: "g1", Name: "Prod"})
	_ = s.CreateMonitor(db.Monitor{ID: "m1", GroupID: "g1", Name: "API", Interval: 60})

	// Create + close an outage so it lands in the History bucket. The CreateOutage helper
	// uses CURRENT_TIMESTAMP, so this row's start_time will be "now".
	if err := s.CreateOutage("m1", "down", "boom"); err != nil {
		t.Fatalf("CreateOutage: %v", err)
	}
	if err := s.CloseOutage("m1"); err != nil {
		t.Fatalf("CloseOutage: %v", err)
	}

	today := time.Now().UTC().Format("2006-01-02")
	yesterday := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")

	// Today: outage should appear.
	req := httptest.NewRequest("GET", "/api/events?date="+today, nil)
	w := httptest.NewRecorder()
	h.GetSystemEvents(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("today: expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse: %v", err)
	}
	history, _ := resp["history"].([]interface{})
	if len(history) != 1 {
		t.Errorf("today: expected 1 history item, got %d (%v)", len(history), resp["history"])
	}

	// Yesterday: nothing should match.
	req2 := httptest.NewRequest("GET", "/api/events?date="+yesterday, nil)
	w2 := httptest.NewRecorder()
	h.GetSystemEvents(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("yesterday: expected 200, got %d", w2.Code)
	}
	var resp2 map[string]interface{}
	_ = json.Unmarshal(w2.Body.Bytes(), &resp2)
	history2, _ := resp2["history"].([]interface{})
	if len(history2) != 0 {
		t.Errorf("yesterday: expected 0 history items, got %d", len(history2))
	}
}

// TestGetSystemEvents_FilterByMonitor verifies ?monitorId= narrows both active outages
// and SSL warnings to the selected monitor. Used by /incidents?monitorId= and by the
// MonitorPage when it rolls up outages for one monitor.
func TestGetSystemEvents_FilterByMonitor(t *testing.T) {
	s, _ := db.NewStore(db.NewTestConfig())
	m := uptime.NewManager(s)
	h := NewEventHandler(s, m)

	_ = s.CreateGroup(db.Group{ID: "g1", Name: "Prod"})
	_ = s.CreateMonitor(db.Monitor{ID: "m-a", GroupID: "g1", Name: "A", Interval: 60})
	_ = s.CreateMonitor(db.Monitor{ID: "m-b", GroupID: "g1", Name: "B", Interval: 60})
	_ = s.CreateOutage("m-a", "down", "A boom")
	_ = s.CreateOutage("m-b", "down", "B boom")

	req := httptest.NewRequest("GET", "/api/events?monitorId=m-a", nil)
	w := httptest.NewRecorder()
	h.GetSystemEvents(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	active := resp["active"].([]interface{})
	if len(active) != 1 {
		t.Fatalf("expected 1 active outage filtered to m-a, got %d", len(active))
	}
	if got := active[0].(map[string]interface{})["monitorId"]; got != "m-a" {
		t.Errorf("expected monitorId=m-a, got %v", got)
	}
}

// TestGetSystemEvents_FilterByGroup verifies ?groupId= narrows outages to monitors that
// belong to the named group. Used by the IncidentsView Group dropdown.
func TestGetSystemEvents_FilterByGroup(t *testing.T) {
	s, _ := db.NewStore(db.NewTestConfig())
	m := uptime.NewManager(s)
	h := NewEventHandler(s, m)

	_ = s.CreateGroup(db.Group{ID: "g-prod", Name: "Prod"})
	_ = s.CreateGroup(db.Group{ID: "g-staging", Name: "Staging"})
	_ = s.CreateMonitor(db.Monitor{ID: "m-prod-1", GroupID: "g-prod", Name: "Prod-1", Interval: 60})
	_ = s.CreateMonitor(db.Monitor{ID: "m-stg-1", GroupID: "g-staging", Name: "Stg-1", Interval: 60})
	_ = s.CreateOutage("m-prod-1", "down", "prod down")
	_ = s.CreateOutage("m-stg-1", "degraded", "stg slow")

	req := httptest.NewRequest("GET", "/api/events?groupId=g-prod", nil)
	w := httptest.NewRecorder()
	h.GetSystemEvents(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	active := resp["active"].([]interface{})
	if len(active) != 1 {
		t.Fatalf("expected 1 active outage filtered to g-prod, got %d", len(active))
	}
	if got := active[0].(map[string]interface{})["groupId"]; got != "g-prod" {
		t.Errorf("expected groupId=g-prod, got %v", got)
	}
}

// TestGetSystemEvents_InvalidDate verifies the handler rejects malformed dates instead of
// silently returning the legacy 7-day view, so Slack links to bogus URLs surface the error.
func TestGetSystemEvents_InvalidDate(t *testing.T) {
	s, _ := db.NewStore(db.NewTestConfig())
	m := uptime.NewManager(s)
	h := NewEventHandler(s, m)

	req := httptest.NewRequest("GET", "/api/events?date=not-a-date", nil)
	w := httptest.NewRecorder()
	h.GetSystemEvents(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid date, got %d body=%s", w.Code, w.Body.String())
	}
}
