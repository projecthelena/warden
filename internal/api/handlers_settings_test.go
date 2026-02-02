package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/clusteruptime/clusteruptime/internal/db"
	"github.com/clusteruptime/clusteruptime/internal/uptime"
)

func TestGetSettings(t *testing.T) {
	s, _ := db.NewStore(db.NewTestConfig())
	m := uptime.NewManager(s)
	h := NewSettingsHandler(s, m)

	req := httptest.NewRequest("GET", "/api/settings", nil)
	w := httptest.NewRecorder()

	h.GetSettings(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Verify default values
	if response["latency_threshold"] != "1000" {
		t.Errorf("Expected latency_threshold '1000', got %s", response["latency_threshold"])
	}
	if response["data_retention_days"] != "30" {
		t.Errorf("Expected data_retention_days '30', got %s", response["data_retention_days"])
	}
}

func TestUpdateSettings_MultipleSettings(t *testing.T) {
	s, _ := db.NewStore(db.NewTestConfig())
	m := uptime.NewManager(s)
	h := NewSettingsHandler(s, m)

	body := map[string]string{
		"latency_threshold":   "500",
		"data_retention_days": "60",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("PATCH", "/api/settings", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateSettings(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	// Verify all settings were saved
	lat, _ := s.GetSetting("latency_threshold")
	if lat != "500" {
		t.Errorf("Expected latency_threshold '500', got %s", lat)
	}

	ret, _ := s.GetSetting("data_retention_days")
	if ret != "60" {
		t.Errorf("Expected data_retention_days '60', got %s", ret)
	}
}

func TestUpdateSettings_InvalidBody(t *testing.T) {
	s, _ := db.NewStore(db.NewTestConfig())
	m := uptime.NewManager(s)
	h := NewSettingsHandler(s, m)

	req := httptest.NewRequest("PATCH", "/api/settings", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateSettings(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid JSON, got %d", w.Code)
	}
}

func TestUpdateSettings_LatencyThresholdUpdatesManager(t *testing.T) {
	s, _ := db.NewStore(db.NewTestConfig())
	m := uptime.NewManager(s)
	h := NewSettingsHandler(s, m)

	body := map[string]string{
		"latency_threshold": "2000",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("PATCH", "/api/settings", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateSettings(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	// Verify manager was updated
	if m.GetLatencyThreshold() != 2000 {
		t.Errorf("Expected latency threshold 2000, got %d", m.GetLatencyThreshold())
	}
}
