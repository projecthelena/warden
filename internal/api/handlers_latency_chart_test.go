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

func TestGetMonitorLatency_ReturnsExplicitChartStates(t *testing.T) {
	_, _, _, _, store := setupTest(t)
	if err := store.CreateMonitor(db.Monitor{ID: "m-chart-api", GroupID: "g-default", Name: "Chart", URL: "https://example.test", Interval: 60, Active: true}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := store.BatchInsertChecks([]db.CheckResult{
		{MonitorID: "m-chart-api", Status: "down", Latency: 15, Timestamp: now.Add(-10 * time.Minute), StatusCode: 503},
	}); err != nil {
		t.Fatal(err)
	}

	handler := NewUptimeHandler(uptime.NewManager(store), store)
	router := chi.NewRouter()
	router.Get("/api/monitors/{id}/latency", handler.GetMonitorLatency)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/monitors/m-chart-api/latency?range=1h", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("got %d: %s", recorder.Code, recorder.Body.String())
	}
	var points []db.LatencyChartPoint
	if err := json.Unmarshal(recorder.Body.Bytes(), &points); err != nil {
		t.Fatal(err)
	}
	if len(points) < 60 {
		t.Fatalf("got %d points, want a complete one-hour timeline", len(points))
	}

	foundDown := false
	foundNoData := false
	for _, point := range points {
		if point.State == "down" {
			foundDown = true
			if point.Latency != nil || point.FailedChecks != 1 {
				t.Fatalf("down bucket must expose failure without response latency: %+v", point)
			}
		}
		if point.State == "no_data" {
			foundNoData = true
		}
	}
	if !foundDown || !foundNoData {
		t.Fatalf("response states: down=%v no_data=%v", foundDown, foundNoData)
	}
}
