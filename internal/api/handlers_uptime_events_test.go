package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/projecthelena/warden/internal/db"
	"github.com/projecthelena/warden/internal/uptime"
)

// GetHistory fetches events for the whole board in one batched query. This checks the
// wiring: each monitor's DTO carries its own events, not another monitor's and not none.
func TestGetHistory_ReturnsEventsPerMonitor(t *testing.T) {
	_, _, _, _, s := setupTest(t)
	manager := uptime.NewManager(s)
	uptimeH := NewUptimeHandler(manager, s)

	if err := s.CreateMonitor(db.Monitor{ID: "m-a", GroupID: "g-default", Name: "A", URL: "http://a", Interval: 60, Active: true}); err != nil {
		t.Fatalf("create m-a: %v", err)
	}
	if err := s.CreateMonitor(db.Monitor{ID: "m-b", GroupID: "g-default", Name: "B", URL: "http://b", Interval: 60, Active: true}); err != nil {
		t.Fatalf("create m-b: %v", err)
	}
	// A has two events, B has one.
	_ = s.CreateEvent("m-a", "down", "a down")
	_ = s.CreateEvent("m-a", "recovered", "a up")
	_ = s.CreateEvent("m-b", "down", "b down")
	manager.Sync()

	req := httptest.NewRequest("GET", "/api/uptime", nil)
	w := httptest.NewRecorder()
	r := chi.NewRouter()
	r.Use(adminRoleMiddleware)
	r.Get("/api/uptime", uptimeH.GetHistory)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse: %v", err)
	}

	counts := map[string]int{}
	for _, g := range resp["groups"].([]interface{}) {
		for _, mon := range g.(map[string]interface{})["monitors"].([]interface{}) {
			m := mon.(map[string]interface{})
			id, _ := m["id"].(string)
			evs, _ := m["events"].([]interface{})
			counts[id] = len(evs)
			for _, e := range evs {
				ev := e.(map[string]interface{})
				// Each event must carry a real message, i.e. the batched rows were scanned,
				// not left as empty placeholders.
				if msg, _ := ev["message"].(string); msg == "" {
					t.Errorf("%s has an event with no message", id)
				}
			}
		}
	}

	if counts["m-a"] != 2 {
		t.Errorf("m-a: expected 2 events, got %d", counts["m-a"])
	}
	if counts["m-b"] != 1 {
		t.Errorf("m-b: expected 1 event, got %d", counts["m-b"])
	}
}
