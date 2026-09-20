package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/projecthelena/warden/internal/db"
	"github.com/projecthelena/warden/internal/uptime"
)

func TestGetGroupMonitorsPaginatesSearchesAndFilters(t *testing.T) {
	_, _, _, _, store := setupTest(t)
	manager := uptime.NewManager(store)
	handler := NewUptimeHandler(manager, store)

	for i := 0; i < 30; i++ {
		name := fmt.Sprintf("Service %02d", i)
		if i == 12 {
			name = "Checkout API"
		}
		if err := store.CreateMonitor(db.Monitor{ID: fmt.Sprintf("m-%02d", i), GroupID: "g-default", Name: name, URL: fmt.Sprintf("https://service-%02d.example.test", i), Interval: 60, Active: true}); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.CreateMonitor(db.Monitor{ID: "m-paused", GroupID: "g-default", Name: "Paused worker", URL: "https://paused.example.test", Interval: 60, Active: false}); err != nil {
		t.Fatal(err)
	}
	manager.Sync()
	manager.GetMonitor("m-00").RecordResult(false, 0, time.Now(), 500, "failed", false)
	manager.GetMonitor("m-01").RecordResult(true, 900, time.Now(), 200, "", true)

	router := chi.NewRouter()
	router.Get("/api/groups/{id}/monitors", handler.GetGroupMonitors)

	t.Run("bounded second page", func(t *testing.T) {
		resp := requestGroupMonitors(t, router, "/api/groups/g-default/monitors?page=2&page_size=10")
		if len(resp.Group.Monitors) != 10 {
			t.Fatalf("expected 10 monitors, got %d", len(resp.Group.Monitors))
		}
		if resp.Pagination.Total != 31 || resp.Pagination.TotalPages != 4 || resp.Pagination.Page != 2 {
			t.Fatalf("unexpected pagination: %+v", resp.Pagination)
		}
		if resp.Counts.Operational != 28 || resp.Counts.Issues != 2 || resp.Counts.Paused != 1 {
			t.Fatalf("unexpected counts: %+v", resp.Counts)
		}
	})

	t.Run("literal case-insensitive search", func(t *testing.T) {
		resp := requestGroupMonitors(t, router, "/api/groups/g-default/monitors?search=CHECKOUT")
		if resp.Pagination.Total != 1 || len(resp.Group.Monitors) != 1 || resp.Group.Monitors[0].ID != "m-12" {
			t.Fatalf("unexpected search result: %+v", resp)
		}
	})

	t.Run("issues filter", func(t *testing.T) {
		resp := requestGroupMonitors(t, router, "/api/groups/g-default/monitors?status=issues")
		if resp.Pagination.Total != 2 || len(resp.Group.Monitors) != 2 {
			t.Fatalf("expected two issues, got %+v", resp.Pagination)
		}
		for _, monitor := range resp.Group.Monitors {
			if monitor.Status != "down" && monitor.Status != "degraded" {
				t.Fatalf("unexpected monitor in issues filter: %s", monitor.Status)
			}
		}
	})
}

func TestGetGroupMonitorsValidatesQueryAndGroup(t *testing.T) {
	_, _, _, _, store := setupTest(t)
	handler := NewUptimeHandler(uptime.NewManager(store), store)
	router := chi.NewRouter()
	router.Get("/api/groups/{id}/monitors", handler.GetGroupMonitors)

	for _, path := range []string{
		"/api/groups/g-default/monitors?page=0",
		"/api/groups/g-default/monitors?page_size=101",
		"/api/groups/g-default/monitors?status=unknown",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != http.StatusBadRequest {
			t.Errorf("%s: expected 400, got %d", path, res.Code)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/groups/missing/monitors", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", res.Code)
	}
}

func TestGetGroupMonitorsBoundsLargeGroupResponse(t *testing.T) {
	_, _, _, _, store := setupTest(t)
	for i := 0; i < 905; i++ {
		if err := store.CreateMonitor(db.Monitor{
			ID: fmt.Sprintf("large-%04d", i), GroupID: "g-default", Name: fmt.Sprintf("Large monitor %04d", i),
			URL: "https://example.test/health", Interval: 60, Active: false,
		}); err != nil {
			t.Fatal(err)
		}
	}
	handler := NewUptimeHandler(uptime.NewManager(store), store)
	router := chi.NewRouter()
	router.Get("/api/groups/{id}/monitors", handler.GetGroupMonitors)

	resp := requestGroupMonitors(t, router, "/api/groups/g-default/monitors?page=1&page_size=25")
	if resp.Pagination.Total != 905 {
		t.Fatalf("expected total 905, got %d", resp.Pagination.Total)
	}
	if len(resp.Group.Monitors) != 25 {
		t.Fatalf("large group response escaped its bound: got %d monitors", len(resp.Group.Monitors))
	}
}

func requestGroupMonitors(t *testing.T, router http.Handler, path string) GroupMonitorsResponse {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	var response GroupMonitorsResponse
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	return response
}
