package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/projecthelena/warden/internal/db"
	"github.com/projecthelena/warden/internal/uptime"
)

func withAdminCtx(req *http.Request) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), contextKeyUserRole, RoleAdmin))
}

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
	if response["data_retention_days"] != "365" {
		t.Errorf("Expected data_retention_days '365', got %s", response["data_retention_days"])
	}
}

func TestUpdateSettings_MultipleSettings(t *testing.T) {
	s, _ := db.NewStore(db.NewTestConfig())
	m := uptime.NewManager(s)
	h := NewSettingsHandler(s, m)

	body := map[string]string{
		"latency_threshold":   "500",
		"data_retention_days": "180",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("PATCH", "/api/settings", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateSettings(w, withAdminCtx(req))

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	// Verify all settings were saved
	lat, _ := s.GetSetting("latency_threshold")
	if lat != "500" {
		t.Errorf("Expected latency_threshold '500', got %s", lat)
	}

	ret, _ := s.GetSetting("data_retention_days")
	if ret != "180" {
		t.Errorf("Expected data_retention_days '180', got %s", ret)
	}
}

func TestUpdateSettings_InvalidBody(t *testing.T) {
	s, _ := db.NewStore(db.NewTestConfig())
	m := uptime.NewManager(s)
	h := NewSettingsHandler(s, m)

	req := httptest.NewRequest("PATCH", "/api/settings", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateSettings(w, withAdminCtx(req))

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

	h.UpdateSettings(w, withAdminCtx(req))

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	// Verify manager was updated
	if m.GetLatencyThreshold() != 2000 {
		t.Errorf("Expected latency threshold 2000, got %d", m.GetLatencyThreshold())
	}
}

// The sustained ladder is only configurable if the settings endpoint actually carries it.
// Defaults matter as much as the stored values: a fresh install must report the ladder it
// is really using, not blanks the UI would then save as zeros.
func TestGetSettings_ReportsTheAlertLadder(t *testing.T) {
	s, _ := db.NewStore(db.NewTestConfig())
	m := uptime.NewManager(s)
	h := NewSettingsHandler(s, m)

	req := httptest.NewRequest("GET", "/api/settings", nil)
	w := httptest.NewRecorder()
	h.GetSettings(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("parse: %v", err)
	}

	for key, want := range map[string]string{
		"notification.alert.sustained_seconds":       "180",
		"notification.alert.reminder_minutes":        "30",
		"notification.alert.repeat_reminder_minutes": "60",
	} {
		if response[key] != want {
			t.Errorf("%s = %q, want the default %q", key, response[key], want)
		}
	}
}

func TestUpdateSettings_PersistsTheAlertLadder(t *testing.T) {
	s, _ := db.NewStore(db.NewTestConfig())
	m := uptime.NewManager(s)
	h := NewSettingsHandler(s, m)

	body, _ := json.Marshal(map[string]string{
		"notification.alert.sustained_seconds":       "300",
		"notification.alert.reminder_minutes":        "15",
		"notification.alert.repeat_reminder_minutes": "120",
	})
	req := withAdminCtx(httptest.NewRequest("PATCH", "/api/settings", bytes.NewReader(body)))
	w := httptest.NewRecorder()
	h.UpdateSettings(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	for key, want := range map[string]string{
		"notification.alert.sustained_seconds":       "300",
		"notification.alert.reminder_minutes":        "15",
		"notification.alert.repeat_reminder_minutes": "120",
	} {
		got, err := s.GetSetting(key)
		if err != nil || got != want {
			t.Errorf("%s = %q (err %v), want %q", key, got, err, want)
		}
	}
}

// Zero is a legal, meaningful value on all three — no silent window, no reminders, no
// repeats — so the validator must not treat it as missing and reject it.
func TestUpdateSettings_AcceptsZeroOnTheAlertLadder(t *testing.T) {
	s, _ := db.NewStore(db.NewTestConfig())
	m := uptime.NewManager(s)
	h := NewSettingsHandler(s, m)

	body, _ := json.Marshal(map[string]string{
		"notification.alert.sustained_seconds": "0",
		"notification.alert.reminder_minutes":  "0",
	})
	req := withAdminCtx(httptest.NewRequest("PATCH", "/api/settings", bytes.NewReader(body)))
	w := httptest.NewRecorder()
	h.UpdateSettings(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("zero was rejected: %d %s", w.Code, w.Body.String())
	}
	if got, _ := s.GetSetting("notification.alert.sustained_seconds"); got != "0" {
		t.Errorf("sustained_seconds = %q, want 0", got)
	}
}

// Nonsense must not reach the manager, where it would silently fall back to the default
// and leave the operator believing they had configured something.
func TestUpdateSettings_RejectsAnOutOfRangeLadder(t *testing.T) {
	s, _ := db.NewStore(db.NewTestConfig())
	m := uptime.NewManager(s)
	h := NewSettingsHandler(s, m)

	for _, tc := range []struct{ key, value string }{
		{"notification.alert.sustained_seconds", "-1"},
		{"notification.alert.sustained_seconds", "999999"},
		{"notification.alert.reminder_minutes", "not-a-number"},
	} {
		body, _ := json.Marshal(map[string]string{tc.key: tc.value})
		req := withAdminCtx(httptest.NewRequest("PATCH", "/api/settings", bytes.NewReader(body)))
		w := httptest.NewRecorder()
		h.UpdateSettings(w, req)

		if w.Code == http.StatusOK {
			t.Errorf("%s=%q was accepted", tc.key, tc.value)
		}
		if got, _ := s.GetSetting(tc.key); got == tc.value {
			t.Errorf("%s=%q was persisted despite being invalid", tc.key, tc.value)
		}
	}
}

// The correlation thresholds are only tunable if the endpoint carries them, and their
// defaults have to be the ones the manager really uses: a blank would be saved back as
// something else the first time anyone touched the form.
func TestGetSettings_ReportsTheCorrelationThresholds(t *testing.T) {
	s, _ := db.NewStore(db.NewTestConfig())
	m := uptime.NewManager(s)
	h := NewSettingsHandler(s, m)

	req := httptest.NewRequest("GET", "/api/settings", nil)
	w := httptest.NewRecorder()
	h.GetSettings(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("parse: %v", err)
	}

	for key, want := range map[string]string{
		"notification.correlation.window_seconds": "300",
		"notification.correlation.min_monitors":   "3",
		"notification.correlation.group_percent":  "30",
		"notification.correlation.probe_percent":  "80",
		"notification.chronic.limit":              "3",
		"notification.chronic.window_minutes":     "1440",
	} {
		if response[key] != want {
			t.Errorf("%s = %q, want the default %q", key, response[key], want)
		}
	}
}

func TestUpdateSettings_PersistsTheCorrelationThresholds(t *testing.T) {
	s, _ := db.NewStore(db.NewTestConfig())
	m := uptime.NewManager(s)
	h := NewSettingsHandler(s, m)

	body, _ := json.Marshal(map[string]string{
		"notification.correlation.window_seconds": "600",
		"notification.correlation.min_monitors":   "5",
		"notification.correlation.group_percent":  "50",
		"notification.chronic.limit":              "0",
	})
	req := withAdminCtx(httptest.NewRequest("PATCH", "/api/settings", bytes.NewReader(body)))
	w := httptest.NewRecorder()
	h.UpdateSettings(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	for key, want := range map[string]string{
		"notification.correlation.window_seconds": "600",
		"notification.correlation.min_monitors":   "5",
		"notification.correlation.group_percent":  "50",
		"notification.chronic.limit":              "0",
	} {
		if got, _ := s.GetSetting(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

// Nonsense here is worse than elsewhere: the manager falls back to its default for a value
// it cannot use, so an accepted bad value leaves the operator believing they widened a
// window that never moved.
func TestUpdateSettings_RejectsBadCorrelationThresholds(t *testing.T) {
	s, _ := db.NewStore(db.NewTestConfig())
	m := uptime.NewManager(s)
	h := NewSettingsHandler(s, m)

	for _, tc := range []struct{ key, value string }{
		{"notification.correlation.group_percent", "0"},
		{"notification.correlation.group_percent", "101"},
		{"notification.correlation.probe_percent", "-5"},
		{"notification.correlation.min_monitors", "0"},
		{"notification.chronic.window_minutes", "0"},
	} {
		body, _ := json.Marshal(map[string]string{tc.key: tc.value})
		req := withAdminCtx(httptest.NewRequest("PATCH", "/api/settings", bytes.NewReader(body)))
		w := httptest.NewRecorder()
		h.UpdateSettings(w, req)

		if w.Code == http.StatusOK {
			t.Errorf("%s=%q was accepted", tc.key, tc.value)
		}
		if got, _ := s.GetSetting(tc.key); got == tc.value {
			t.Errorf("%s=%q was persisted despite being invalid", tc.key, tc.value)
		}
	}
}
