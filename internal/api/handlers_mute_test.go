package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/projecthelena/warden/internal/db"
)

func viewerRoleMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), contextKeyUserRole, RoleViewer)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func muteRouter(crudH *CRUDHandler) http.Handler {
	r := chi.NewRouter()
	r.Use(adminRoleMiddleware)
	r.Post("/api/monitors/{id}/alerts", crudH.SetMonitorAlerts)
	return r
}

func postMute(t *testing.T, h http.Handler, id string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var payload []byte
	switch v := body.(type) {
	case string:
		payload = []byte(v)
	default:
		payload, _ = json.Marshal(v)
	}
	req := httptest.NewRequest("POST", "/api/monitors/"+id+"/alerts", bytes.NewBuffer(payload))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestSetMonitorAlerts_MutesAndUnmutes(t *testing.T) {
	crudH, _, _, _, s := setupTest(t)
	if err := s.CreateMonitor(db.Monitor{ID: "m1", GroupID: "g-default", Name: "Staging", URL: "http://x.com", Interval: 60}); err != nil {
		t.Fatalf("CreateMonitor: %v", err)
	}
	h := muteRouter(crudH)

	w := postMute(t, h, "m1", map[string]bool{"muted": true})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if resp["alertsMuted"] != true {
		t.Errorf("response does not confirm the new state: %v", resp)
	}

	monitors, _ := s.GetMonitors()
	if !monitors[0].AlertsMuted {
		t.Fatal("the mute did not persist")
	}

	if w := postMute(t, h, "m1", map[string]bool{"muted": false}); w.Code != http.StatusOK {
		t.Fatalf("unmute: expected 200, got %d", w.Code)
	}
	monitors, _ = s.GetMonitors()
	if monitors[0].AlertsMuted {
		t.Error("the unmute did not persist")
	}
}

// Muting is not pausing. A muted monitor keeps being checked, which is the whole point of
// having a separate control for it.
func TestSetMonitorAlerts_DoesNotPause(t *testing.T) {
	crudH, _, _, _, s := setupTest(t)
	if err := s.CreateMonitor(db.Monitor{ID: "m1", GroupID: "g-default", Name: "Staging", URL: "http://x.com", Active: true, Interval: 60}); err != nil {
		t.Fatalf("CreateMonitor: %v", err)
	}

	if w := postMute(t, muteRouter(crudH), "m1", map[string]bool{"muted": true}); w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	monitors, _ := s.GetMonitors()
	if !monitors[0].Active {
		t.Error("muting paused the monitor")
	}
}

func TestSetMonitorAlerts_UnknownMonitor(t *testing.T) {
	crudH, _, _, _, _ := setupTest(t)
	if w := postMute(t, muteRouter(crudH), "does-not-exist", map[string]bool{"muted": true}); w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestSetMonitorAlerts_MalformedBody(t *testing.T) {
	crudH, _, _, _, s := setupTest(t)
	if err := s.CreateMonitor(db.Monitor{ID: "m1", GroupID: "g-default", Name: "Staging", URL: "http://x.com", Interval: 60}); err != nil {
		t.Fatalf("CreateMonitor: %v", err)
	}

	if w := postMute(t, muteRouter(crudH), "m1", "not json"); w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	// An empty body is malformed too, and must not be read as "unmute".
	if err := s.SetMonitorAlertsMuted("m1", true); err != nil {
		t.Fatalf("SetMonitorAlertsMuted: %v", err)
	}
	if w := postMute(t, muteRouter(crudH), "m1", ""); w.Code != http.StatusBadRequest {
		t.Errorf("empty body: expected 400, got %d", w.Code)
	}
	monitors, _ := s.GetMonitors()
	if !monitors[0].AlertsMuted {
		t.Error("a rejected request changed the mute anyway")
	}
}

// Muting silences a monitor, so it is an editor action rather than something a viewer can
// do. Without this the role check could be dropped and every other test would stay green.
func TestSetMonitorAlerts_RequiresEditor(t *testing.T) {
	crudH, _, _, _, s := setupTest(t)
	if err := s.CreateMonitor(db.Monitor{ID: "m1", GroupID: "g-default", Name: "Staging", URL: "http://x.com", Interval: 60}); err != nil {
		t.Fatalf("CreateMonitor: %v", err)
	}

	r := chi.NewRouter()
	r.Use(viewerRoleMiddleware)
	r.Post("/api/monitors/{id}/alerts", crudH.SetMonitorAlerts)

	req := httptest.NewRequest("POST", "/api/monitors/m1/alerts", bytes.NewBufferString(`{"muted":true}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("a viewer got %d, want 403", w.Code)
	}
	monitors, _ := s.GetMonitors()
	if monitors[0].AlertsMuted {
		t.Error("a viewer managed to mute a monitor")
	}
}
