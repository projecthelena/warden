package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/projecthelena/warden/internal/db"
	"github.com/projecthelena/warden/internal/uptime"
)

func TestCreateMaintenance(t *testing.T) {
	s, _ := db.NewStore(db.NewTestConfig())
	m := uptime.NewManager(s)
	h := NewMaintenanceHandler(s, m)

	// Seed Group
	if err := s.CreateGroup(db.Group{ID: "g1", Name: "G1"}); err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}

	payload := map[string]interface{}{
		"title":       "Upgrade",
		"description": "Upgrading DB",
		"startTime":   time.Now().Format(time.RFC3339),
		"endTime":     time.Now().Add(1 * time.Hour).Format(time.RFC3339),
		"groupIds":    []string{"g1"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/maintenance", bytes.NewBuffer(body))
	req = req.WithContext(context.WithValue(req.Context(), contextKeyUserRole, RoleAdmin))
	w := httptest.NewRecorder()

	h.CreateMaintenance(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected 201 Created, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestGetMaintenance(t *testing.T) {
	s, _ := db.NewStore(db.NewTestConfig())
	m := uptime.NewManager(s)
	h := NewMaintenanceHandler(s, m)

	req := httptest.NewRequest("GET", "/api/maintenance", nil)
	w := httptest.NewRecorder()

	h.GetMaintenance(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

func TestCreateMaintenanceRejectsInvalidTimeRange(t *testing.T) {
	s, _ := db.NewStore(db.NewTestConfig())
	h := NewMaintenanceHandler(s, uptime.NewManager(s))

	payload := map[string]interface{}{
		"title":          "Invalid window",
		"startTime":      "2026-09-15T18:00:00-05:00",
		"endTime":        "2026-09-15T17:00:00-05:00",
		"affectedGroups": []string{"g1"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/maintenance", bytes.NewBuffer(body))
	req = req.WithContext(context.WithValue(req.Context(), contextKeyUserRole, RoleAdmin))
	w := httptest.NewRecorder()

	h.CreateMaintenance(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d", w.Code)
	}
}

func TestDeleteMaintenanceDoesNotDeleteIncident(t *testing.T) {
	s, _ := db.NewStore(db.NewTestConfig())
	h := NewMaintenanceHandler(s, uptime.NewManager(s))
	incident := db.Incident{
		ID: "incident-1", Title: "Outage", Type: "incident", Severity: "major",
		Status: "investigating", StartTime: time.Now(), AffectedGroups: "[]",
	}
	if err := s.CreateIncident(incident); err != nil {
		t.Fatal(err)
	}

	r := chi.NewRouter()
	r.Delete("/api/maintenance/{id}", h.DeleteMaintenance)
	req := httptest.NewRequest("DELETE", "/api/maintenance/incident-1", nil)
	req = req.WithContext(context.WithValue(req.Context(), contextKeyUserRole, RoleAdmin))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found, got %d", w.Code)
	}
	if got, err := s.GetIncidentByID(incident.ID); err != nil || got == nil {
		t.Fatal("incident was deleted through the maintenance endpoint")
	}
}
