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
	s, _ := db.NewStore(":memory:")
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
	if response["ssl_expiry_threshold_days"] != "30" {
		t.Errorf("Expected ssl_expiry_threshold_days '30', got %s", response["ssl_expiry_threshold_days"])
	}
}

func TestGetSettings_IncludesNotificationTimezone(t *testing.T) {
	s, _ := db.NewStore(":memory:")
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

	// notification_timezone should be present
	if _, ok := response["notification_timezone"]; !ok {
		t.Error("Expected notification_timezone in response")
	}
	// Default should be empty or "UTC"
	if response["notification_timezone"] != "" && response["notification_timezone"] != "UTC" {
		t.Errorf("Expected notification_timezone default 'UTC' or empty, got %s", response["notification_timezone"])
	}
}

func TestUpdateSettings_NotificationTimezone_Valid(t *testing.T) {
	s, _ := db.NewStore(":memory:")
	m := uptime.NewManager(s)
	h := NewSettingsHandler(s, m)

	body := map[string]string{
		"notification_timezone": "America/New_York",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("PATCH", "/api/settings", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateSettings(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the setting was saved
	saved, err := s.GetSetting("notification_timezone")
	if err != nil {
		t.Fatalf("GetSetting failed: %v", err)
	}
	if saved != "America/New_York" {
		t.Errorf("Expected 'America/New_York', got %s", saved)
	}
}

func TestUpdateSettings_NotificationTimezone_Invalid(t *testing.T) {
	s, _ := db.NewStore(":memory:")
	m := uptime.NewManager(s)
	h := NewSettingsHandler(s, m)

	body := map[string]string{
		"notification_timezone": "Invalid/Timezone",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("PATCH", "/api/settings", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateSettings(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid timezone, got %d", w.Code)
	}
}

func TestUpdateSettings_NotificationTimezone_UTC(t *testing.T) {
	s, _ := db.NewStore(":memory:")
	m := uptime.NewManager(s)
	h := NewSettingsHandler(s, m)

	body := map[string]string{
		"notification_timezone": "UTC",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("PATCH", "/api/settings", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateSettings(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	saved, _ := s.GetSetting("notification_timezone")
	if saved != "UTC" {
		t.Errorf("Expected 'UTC', got %s", saved)
	}
}

func TestUpdateSettings_NotificationTimezone_VariousTimezones(t *testing.T) {
	validTimezones := []string{
		"UTC",
		"America/New_York",
		"America/Los_Angeles",
		"Europe/London",
		"Europe/Paris",
		"Asia/Tokyo",
		"Australia/Sydney",
		"Pacific/Auckland",
	}

	for _, tz := range validTimezones {
		t.Run(tz, func(t *testing.T) {
			s, _ := db.NewStore(":memory:")
			m := uptime.NewManager(s)
			h := NewSettingsHandler(s, m)

			body := map[string]string{
				"notification_timezone": tz,
			}
			bodyBytes, _ := json.Marshal(body)

			req := httptest.NewRequest("PATCH", "/api/settings", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			h.UpdateSettings(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected 200 for timezone %s, got %d", tz, w.Code)
			}
		})
	}
}

func TestUpdateSettings_MultipleSettings(t *testing.T) {
	s, _ := db.NewStore(":memory:")
	m := uptime.NewManager(s)
	h := NewSettingsHandler(s, m)

	body := map[string]string{
		"latency_threshold":         "500",
		"data_retention_days":       "60",
		"ssl_expiry_threshold_days": "14",
		"notification_timezone":     "Europe/London",
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

	ssl, _ := s.GetSetting("ssl_expiry_threshold_days")
	if ssl != "14" {
		t.Errorf("Expected ssl_expiry_threshold_days '14', got %s", ssl)
	}

	tz, _ := s.GetSetting("notification_timezone")
	if tz != "Europe/London" {
		t.Errorf("Expected notification_timezone 'Europe/London', got %s", tz)
	}
}

func TestUpdateSettings_InvalidBody(t *testing.T) {
	s, _ := db.NewStore(":memory:")
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

func TestUpdateSettings_SSLThresholdUpdatesManager(t *testing.T) {
	s, _ := db.NewStore(":memory:")
	m := uptime.NewManager(s)
	h := NewSettingsHandler(s, m)

	// Initial threshold
	if m.GetSSLExpiryThreshold() != 30 {
		t.Fatalf("Expected initial SSL threshold 30, got %d", m.GetSSLExpiryThreshold())
	}

	body := map[string]string{
		"ssl_expiry_threshold_days": "7",
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
	if m.GetSSLExpiryThreshold() != 7 {
		t.Errorf("Expected SSL threshold 7 in manager, got %d", m.GetSSLExpiryThreshold())
	}
}

func TestUpdateSettings_LatencyThresholdUpdatesManager(t *testing.T) {
	s, _ := db.NewStore(":memory:")
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
