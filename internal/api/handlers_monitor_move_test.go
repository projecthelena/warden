package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/projecthelena/warden/internal/db"
)

func moveRouter(crudH *CRUDHandler) http.Handler {
	r := chi.NewRouter()
	r.Use(adminRoleMiddleware)
	r.Post("/api/monitors/{id}/group", crudH.SetMonitorGroup)
	return r
}

func postMove(t *testing.T, h http.Handler, id string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var payload []byte
	switch v := body.(type) {
	case string:
		payload = []byte(v)
	default:
		payload, _ = json.Marshal(v)
	}
	req := httptest.NewRequest("POST", "/api/monitors/"+id+"/group", bytes.NewBuffer(payload))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func monitorByID(t *testing.T, s *db.Store, id string) db.Monitor {
	t.Helper()
	monitors, err := s.GetMonitors()
	if err != nil {
		t.Fatalf("GetMonitors: %v", err)
	}
	for _, m := range monitors {
		if m.ID == id {
			return m
		}
	}
	t.Fatalf("monitor %s not found", id)
	return db.Monitor{}
}

func seedMovable(t *testing.T, s *db.Store) {
	t.Helper()
	if err := s.CreateGroup(db.Group{ID: "g-other", Name: "Other"}); err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if err := s.CreateMonitor(db.Monitor{ID: "m1", GroupID: "g-default", Name: "M1", URL: "http://x.com", Interval: 60}); err != nil {
		t.Fatalf("CreateMonitor: %v", err)
	}
}

func TestSetMonitorGroup_Moves(t *testing.T) {
	crudH, _, _, _, s := setupTest(t)
	seedMovable(t, s)

	w := postMove(t, moveRouter(crudH), "m1", map[string]string{"groupId": "g-other"})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if out["groupId"] != "g-other" {
		t.Errorf("expected the response to name the destination, got %v", out)
	}
	if got := monitorByID(t, s, "m1").GroupID; got != "g-other" {
		t.Errorf("expected monitor in g-other, got %s", got)
	}
}

// The move is a reassignment, not a rewrite: everything else about the monitor has to
// come out the other side untouched.
func TestSetMonitorGroup_LeavesTheRestOfTheMonitorAlone(t *testing.T) {
	crudH, _, _, _, s := setupTest(t)
	if err := s.CreateGroup(db.Group{ID: "g-other", Name: "Other"}); err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	threshold := 5
	if err := s.CreateMonitor(db.Monitor{
		ID: "m1", GroupID: "g-default", Name: "M1", URL: "db.example.com:5432",
		Type: db.MonitorTypeTCP, Interval: 300, Active: false, AlertsMuted: true,
		ConfirmationThreshold: &threshold,
		RequestConfig:         &db.RequestConfig{TimeoutSeconds: 9},
	}); err != nil {
		t.Fatalf("CreateMonitor: %v", err)
	}

	if w := postMove(t, moveRouter(crudH), "m1", map[string]string{"groupId": "g-other"}); w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	m := monitorByID(t, s, "m1")
	switch {
	case m.GroupID != "g-other":
		t.Errorf("group not applied, got %s", m.GroupID)
	case m.Name != "M1" || m.URL != "db.example.com:5432":
		t.Errorf("identity changed: %+v", m)
	case db.NormalizeMonitorType(m.Type) != db.MonitorTypeTCP:
		t.Errorf("check type changed to %s", m.Type)
	case m.Interval != 300:
		t.Errorf("interval changed to %d", m.Interval)
	case m.Active:
		t.Error("a paused monitor came back active")
	case !m.AlertsMuted:
		t.Error("a muted monitor came back audible")
	case m.ConfirmationThreshold == nil || *m.ConfirmationThreshold != 5:
		t.Errorf("confirmation threshold lost: %v", m.ConfirmationThreshold)
	case m.RequestConfig == nil || m.RequestConfig.TimeoutSeconds != 9:
		t.Errorf("request config lost: %+v", m.RequestConfig)
	}
}

// Moving a monitor where it already is is a no-op, not an error: a client that resends
// the same destination should not have to special-case it.
func TestSetMonitorGroup_SameGroupSucceeds(t *testing.T) {
	crudH, _, _, _, s := setupTest(t)
	seedMovable(t, s)

	if w := postMove(t, moveRouter(crudH), "m1", map[string]string{"groupId": "g-default"}); w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d. Body: %s", w.Code, w.Body.String())
	}
	if got := monitorByID(t, s, "m1").GroupID; got != "g-default" {
		t.Errorf("expected monitor to stay in g-default, got %s", got)
	}
}

func TestSetMonitorGroup_Rejections(t *testing.T) {
	tests := []struct {
		name     string
		monitor  string
		body     any
		expected int
	}{
		{"unknown group", "m1", map[string]string{"groupId": "g-nope"}, http.StatusNotFound},
		{"unknown monitor", "m-nope", map[string]string{"groupId": "g-other"}, http.StatusNotFound},
		{"missing groupId", "m1", map[string]string{}, http.StatusBadRequest},
		{"empty groupId", "m1", map[string]string{"groupId": ""}, http.StatusBadRequest},
		{"malformed body", "m1", "{not json", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			crudH, _, _, _, s := setupTest(t)
			seedMovable(t, s)

			w := postMove(t, moveRouter(crudH), tt.monitor, tt.body)
			if w.Code != tt.expected {
				t.Errorf("got %d, want %d. Body: %s", w.Code, tt.expected, w.Body.String())
			}
			// Whatever the failure, the monitor must not have wandered off.
			if got := monitorByID(t, s, "m1").GroupID; got != "g-default" {
				t.Errorf("monitor moved despite the failure, now in %s", got)
			}
		})
	}
}

// Regrouping a monitor changes what the dashboard and every group-scoped status page
// show, so it is an editor action. Without this the role check could be dropped and
// every other test here would stay green.
func TestSetMonitorGroup_RequiresEditor(t *testing.T) {
	crudH, _, _, _, s := setupTest(t)
	seedMovable(t, s)

	r := chi.NewRouter()
	r.Use(viewerRoleMiddleware)
	r.Post("/api/monitors/{id}/group", crudH.SetMonitorGroup)

	req := httptest.NewRequest("POST", "/api/monitors/m1/group", bytes.NewBufferString(`{"groupId":"g-other"}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("a viewer got %d, want 403", w.Code)
	}
	if got := monitorByID(t, s, "m1").GroupID; got != "g-default" {
		t.Errorf("a viewer managed to move a monitor into %s", got)
	}
}

func TestMoveMonitor(t *testing.T) {
	crudH, _, _, _, s := setupTest(t)
	seedMovable(t, s)

	if err := crudH.MoveMonitor("m1", "g-other"); err != nil {
		t.Fatalf("MoveMonitor failed: %v", err)
	}
	if got := monitorByID(t, s, "m1").GroupID; got != "g-other" {
		t.Errorf("expected monitor in g-other, got %s", got)
	}

	if err := crudH.MoveMonitor("m1", "g-nope"); err == nil {
		t.Error("expected an error moving to an unknown group")
	}
	if err := crudH.MoveMonitor("m1", ""); err == nil {
		t.Error("expected an error moving with no destination")
	}
	if err := crudH.MoveMonitor("", "g-other"); err == nil {
		t.Error("expected an error moving without a monitor id")
	}
}
