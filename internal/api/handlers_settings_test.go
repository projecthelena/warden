package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
	if response["notification.latency.increase_percent"] != "50" {
		t.Errorf("Expected notification.latency.increase_percent '50', got %s", response["notification.latency.increase_percent"])
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
		"notification.latency.increase_percent": "500",
		"data_retention_days":                   "180",
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
	lat, _ := s.GetSetting("notification.latency.increase_percent")
	if lat != "500" {
		t.Errorf("Expected notification.latency.increase_percent '500', got %s", lat)
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

	if err := s.CreateMonitor(db.Monitor{ID: "latency-settings", GroupID: "g-default", Name: "API", URL: "https://example.com", Active: true, Interval: 60}); err != nil {
		t.Fatal(err)
	}
	checks := make([]db.CheckResult, 200)
	for i := range checks {
		checks[i] = db.CheckResult{MonitorID: "latency-settings", Status: "up", Latency: 1000, Timestamp: time.Now().Add(-time.Duration(i) * time.Minute)}
	}
	if err := s.BatchInsertChecks(checks); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ComputeLatencyBaseline("latency-settings", 7*24*time.Hour, 200, time.Now()); err != nil {
		t.Fatal(err)
	}
	m.Sync()
	defer m.Stop()

	body := map[string]string{
		"notification.latency.increase_percent": "2000",
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
	if m.GetMonitor("latency-settings").GetLatencyThreshold() != 21000 {
		t.Errorf("Expected latency threshold 21000, got %d", m.GetMonitor("latency-settings").GetLatencyThreshold())
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
		"notification.event.flapping.enabled":        "false",
		"notification.alert.sustained_seconds":       "180",
		"notification.alert.reminder_minutes":        "0",
		"notification.alert.repeat_reminder_minutes": "0",
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

// The adaptive latency settings are only reachable if the endpoint carries them, and the
// defaults have to match what baseline.go really uses.
func TestGetSettings_ReportsTheLatencyBaseline(t *testing.T) {
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
		"notification.latency.baseline_days":    "7",
		"notification.latency.min_samples":      "200",
		"notification.latency.increase_percent": "50",
	} {
		if response[key] != want {
			t.Errorf("%s = %q, want the default %q", key, response[key], want)
		}
	}
}

func TestUpdateSettings_PersistsTheLatencyBaseline(t *testing.T) {
	s, _ := db.NewStore(db.NewTestConfig())
	m := uptime.NewManager(s)
	h := NewSettingsHandler(s, m)

	body, _ := json.Marshal(map[string]string{
		"notification.latency.baseline_days":    "14",
		"notification.latency.increase_percent": "200",
	})
	req := withAdminCtx(httptest.NewRequest("PATCH", "/api/settings", bytes.NewReader(body)))
	w := httptest.NewRecorder()
	h.UpdateSettings(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	for key, want := range map[string]string{
		"notification.latency.baseline_days":    "14",
		"notification.latency.increase_percent": "200",
	} {
		if got, _ := s.GetSetting(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestRelearnLatencyBaselines_AdminOnly(t *testing.T) {
	s, _ := db.NewStore(db.NewTestConfig())
	m := uptime.NewManager(s)
	h := NewSettingsHandler(s, m)

	for _, tc := range []struct {
		name string
		req  *http.Request
		want int
	}{
		{name: "anonymous", req: httptest.NewRequest("POST", "/api/settings/latency-baselines/relearn", nil), want: http.StatusForbidden},
		{name: "admin", req: withAdminCtx(httptest.NewRequest("POST", "/api/settings/latency-baselines/relearn", nil)), want: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			h.RelearnLatencyBaselines(w, tc.req)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", w.Code, tc.want, w.Body.String())
			}
		})
	}
	if cutoff, err := s.GetSetting("notification.latency.learn_after"); err != nil || cutoff == "" {
		t.Fatalf("learn_after = %q, err=%v", cutoff, err)
	}
}

// Invalid percentages must never be persisted.
func TestUpdateSettings_RejectsBadLatencyBaseline(t *testing.T) {
	s, _ := db.NewStore(db.NewTestConfig())
	m := uptime.NewManager(s)
	h := NewSettingsHandler(s, m)

	for _, tc := range []struct{ key, value string }{
		{"notification.latency.increase_percent", "0"},
		{"notification.latency.increase_percent", "-1"},
		{"notification.latency.increase_percent", "9901"},
		{"notification.latency.increase_percent", "25.5"},
		{"latency_threshold", "1000"},
		{"notification.latency.adaptive_enabled", "false"},
		{"notification.latency.factor_percent", "150"},
		{"notification.latency.baseline_days", "0"},
		{"notification.latency.baseline_days", "365"},
		{"notification.latency.min_samples", "0"},
		{"notification.latency.floor_ms", "-1"},
	} {
		previous, _ := s.GetSetting(tc.key)
		body, _ := json.Marshal(map[string]string{tc.key: tc.value})
		req := withAdminCtx(httptest.NewRequest("PATCH", "/api/settings", bytes.NewReader(body)))
		w := httptest.NewRecorder()
		h.UpdateSettings(w, req)

		if w.Code == http.StatusOK {
			t.Errorf("%s=%q was accepted", tc.key, tc.value)
		}
		if got, _ := s.GetSetting(tc.key); got != previous {
			t.Errorf("%s=%q was persisted despite being invalid", tc.key, tc.value)
		}
	}
}

// The weekly summary is opt-in, and the endpoint has to say so: a default of "true" here
// would start sending a new kind of message to every install that upgrades.
func TestGetSettings_ReportsTheWeeklySummary(t *testing.T) {
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

	if response["notification.insights.weekly_enabled"] != "false" {
		t.Errorf("the weekly summary defaults to %q, want false", response["notification.insights.weekly_enabled"])
	}
	if response["notification.insights.weekly_day"] != "1" {
		t.Errorf("weekly_day = %q, want 1 (Monday)", response["notification.insights.weekly_day"])
	}
	if response["notification.insights.weekly_time"] != "09:00" {
		t.Errorf("weekly_time = %q, want 09:00", response["notification.insights.weekly_time"])
	}
	if response["workspace.timezone"] != "UTC" {
		t.Errorf("workspace.timezone = %q, want UTC", response["workspace.timezone"])
	}
}

func TestUpdateSettings_ValidatesWorkspaceTimezone(t *testing.T) {
	s, _ := db.NewStore(db.NewTestConfig())
	m := uptime.NewManager(s)
	h := NewSettingsHandler(s, m)

	body, _ := json.Marshal(map[string]string{"workspace.timezone": "America/Bogota"})
	w := httptest.NewRecorder()
	h.UpdateSettings(w, withAdminCtx(httptest.NewRequest("PATCH", "/api/settings", bytes.NewReader(body))))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if got, _ := s.GetSetting("workspace.timezone"); got != "America/Bogota" {
		t.Errorf("workspace.timezone = %q, want America/Bogota", got)
	}

	body, _ = json.Marshal(map[string]string{"workspace.timezone": "Invalid/Timezone"})
	w = httptest.NewRecorder()
	h.UpdateSettings(w, withAdminCtx(httptest.NewRequest("PATCH", "/api/settings", bytes.NewReader(body))))
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestUpdateSettings_PersistsTheWeeklySummary(t *testing.T) {
	s, _ := db.NewStore(db.NewTestConfig())
	m := uptime.NewManager(s)
	h := NewSettingsHandler(s, m)

	body, _ := json.Marshal(map[string]string{
		"notification.insights.weekly_enabled": "true",
		"notification.insights.weekly_day":     "5",
		"notification.insights.weekly_time":    "17:30",
	})
	req := withAdminCtx(httptest.NewRequest("PATCH", "/api/settings", bytes.NewReader(body)))
	w := httptest.NewRecorder()
	h.UpdateSettings(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	for key, want := range map[string]string{
		"notification.insights.weekly_enabled": "true",
		"notification.insights.weekly_day":     "5",
		"notification.insights.weekly_time":    "17:30",
	} {
		if got, _ := s.GetSetting(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

// Sunday is 0 and Saturday is 6. A day outside that range would make the summary never
// fire, silently, since the manager clamps it away.
func TestUpdateSettings_RejectsABadWeekday(t *testing.T) {
	s, _ := db.NewStore(db.NewTestConfig())
	m := uptime.NewManager(s)
	h := NewSettingsHandler(s, m)

	for _, value := range []string{"7", "-1"} {
		body, _ := json.Marshal(map[string]string{"notification.insights.weekly_day": value})
		req := withAdminCtx(httptest.NewRequest("PATCH", "/api/settings", bytes.NewReader(body)))
		w := httptest.NewRecorder()
		h.UpdateSettings(w, req)

		if w.Code == http.StatusOK {
			t.Errorf("weekly_day=%q was accepted", value)
		}
	}

	// Both ends of the valid range are legal.
	for _, value := range []string{"0", "6"} {
		body, _ := json.Marshal(map[string]string{"notification.insights.weekly_day": value})
		req := withAdminCtx(httptest.NewRequest("PATCH", "/api/settings", bytes.NewReader(body)))
		w := httptest.NewRecorder()
		h.UpdateSettings(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("weekly_day=%q was rejected: %d", value, w.Code)
		}
	}
}

func patchSettings(t *testing.T, h *SettingsHandler, body map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("PATCH", "/api/settings", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.UpdateSettings(w, withAdminCtx(req))
	return w
}

func TestRollupRetentionSetting(t *testing.T) {
	s, _ := db.NewStore(db.NewTestConfig())
	h := NewSettingsHandler(s, uptime.NewManager(s))
	if err := s.UpsertStatusPageFull(db.StatusPageInput{Slug: "ninety", Title: "Ninety", Public: true, Enabled: true, UptimeDaysRange: 90}); err != nil {
		t.Fatal(err)
	}

	// Unset: GET reports it empty and rollups follow check retention, so a short check
	// retention is still refused while a status page shows 90 days.
	w := httptest.NewRecorder()
	h.GetSettings(w, httptest.NewRequest("GET", "/api/settings", nil))
	var settings map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &settings)
	if v, ok := settings["rollup_retention_days"]; !ok || v != "" {
		t.Fatalf("rollup_retention_days = %q (present=%v), want empty", v, ok)
	}
	if w := patchSettings(t, h, map[string]string{"data_retention_days": "7"}); w.Code != http.StatusBadRequest {
		t.Fatalf("7-day retention with a 90-day page: got %d, want 400", w.Code)
	}

	// Rollups kept for a year let raw checks drop to a week.
	if w := patchSettings(t, h, map[string]string{"data_retention_days": "7", "rollup_retention_days": "365"}); w.Code != http.StatusOK {
		t.Fatalf("split retention: got %d: %s", w.Code, w.Body.String())
	}
	if checks, rollups := s.RetentionDays(); checks != 7 || rollups != 365 {
		t.Fatalf("RetentionDays = (%d, %d), want (7, 365)", checks, rollups)
	}

	for _, body := range []map[string]string{
		{"rollup_retention_days": "30"},  // below the status page range
		{"rollup_retention_days": "5"},   // below check retention
		{"rollup_retention_days": "abc"}, // not a number
		{"rollup_retention_days": "4000"},
	} {
		if w := patchSettings(t, h, body); w.Code != http.StatusBadRequest {
			t.Errorf("%v: got %d, want 400", body, w.Code)
		}
	}

	// Clearing it makes rollups follow check retention again, which the page forbids.
	if w := patchSettings(t, h, map[string]string{"rollup_retention_days": ""}); w.Code != http.StatusBadRequest {
		t.Fatalf("clearing with 7-day checks and a 90-day page: got %d, want 400", w.Code)
	}
	if w := patchSettings(t, h, map[string]string{"data_retention_days": "90", "rollup_retention_days": ""}); w.Code != http.StatusOK {
		t.Fatalf("clear rollup retention: got %d: %s", w.Code, w.Body.String())
	}
	if checks, rollups := s.RetentionDays(); checks != 90 || rollups != 90 {
		t.Fatalf("RetentionDays after clear = (%d, %d), want (90, 90)", checks, rollups)
	}

	// data_retention_days is bounded like the prune functions that use it.
	for _, val := range []string{"0", "3651", "abc"} {
		if w := patchSettings(t, h, map[string]string{"data_retention_days": val}); w.Code != http.StatusBadRequest {
			t.Errorf("data_retention_days %q: got %d, want 400", val, w.Code)
		}
	}

	// Raising check retention past the stored daily history value clears it, so what GET
	// returns and what is stored both match what is in effect.
	if w := patchSettings(t, h, map[string]string{"rollup_retention_days": "180"}); w.Code != http.StatusOK {
		t.Fatalf("set 180-day history: got %d: %s", w.Code, w.Body.String())
	}
	for _, checks := range []string{"120", "400"} {
		if w := patchSettings(t, h, map[string]string{"data_retention_days": checks}); w.Code != http.StatusOK {
			t.Fatalf("raise checks to %s: got %d: %s", checks, w.Code, w.Body.String())
		}
	}
	if stored, _ := s.GetSetting("rollup_retention_days"); stored != "" {
		t.Fatalf("stored rollup_retention_days = %q, want cleared", stored)
	}
	w = httptest.NewRecorder()
	h.GetSettings(w, httptest.NewRequest("GET", "/api/settings", nil))
	settings = map[string]string{}
	_ = json.Unmarshal(w.Body.Bytes(), &settings)
	if settings["data_retention_days"] != "400" || settings["rollup_retention_days"] != "" {
		t.Fatalf("GET after raising checks = (%q, %q), want (400, empty)", settings["data_retention_days"], settings["rollup_retention_days"])
	}

	// A value equal to check retention means the same as empty.
	if w := patchSettings(t, h, map[string]string{"rollup_retention_days": "400"}); w.Code != http.StatusOK {
		t.Fatalf("history equal to checks: got %d: %s", w.Code, w.Body.String())
	}
	if stored, _ := s.GetSetting("rollup_retention_days"); stored != "" {
		t.Fatalf("stored rollup_retention_days = %q, want empty when equal to checks", stored)
	}
}
