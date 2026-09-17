package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/projecthelena/warden/internal/db"
	"github.com/projecthelena/warden/internal/uptime"
)

func TestGetMonitorUptimeReturnsSharedWindowContract(t *testing.T) {
	_, _, _, _, store := setupTest(t)
	if err := store.CreateMonitor(db.Monitor{ID: "m-window", GroupID: "g-default", Name: "Window", URL: "https://example.com", Active: true, Interval: 120}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := store.BatchInsertChecks([]db.CheckResult{
		{MonitorID: "m-window", Status: "up", Timestamp: now.Add(-2 * time.Minute)},
		{MonitorID: "m-window", Status: "down", Timestamp: now.Add(-time.Minute)},
	}); err != nil {
		t.Fatal(err)
	}

	handler := NewUptimeHandler(uptime.NewManager(store), store)
	router := chi.NewRouter()
	router.Get("/api/monitors/{id}/uptime", handler.GetMonitorUptime)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/monitors/m-window/uptime", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body struct {
		Uptime24h float64 `json:"uptime24h"`
		Windows   struct {
			Last24Hours db.UptimeWindow `json:"last24Hours"`
		} `json:"windows"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Uptime24h != 50 || body.Windows.Last24Hours.Percent != 50 {
		t.Fatalf("legacy and shared percentages diverged: %#v", body)
	}
	if body.Windows.Last24Hours.TotalChecks != 2 || body.Windows.Last24Hours.DownChecks != 1 || body.Windows.Last24Hours.DowntimeSeconds != 120 {
		t.Fatalf("unexpected 24h window: %+v", body.Windows.Last24Hours)
	}
}
