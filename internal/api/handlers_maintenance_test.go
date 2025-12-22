package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/clusteruptime/clusteruptime/internal/db"
	"github.com/clusteruptime/clusteruptime/internal/uptime"
)

func TestCreateMaintenance(t *testing.T) {
	s, _ := db.NewStore(":memory:")
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
	w := httptest.NewRecorder()

	h.CreateMaintenance(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected 201 Created, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestGetMaintenance(t *testing.T) {
	s, _ := db.NewStore(":memory:")
	m := uptime.NewManager(s)
	h := NewMaintenanceHandler(s, m)

	req := httptest.NewRequest("GET", "/api/maintenance", nil)
	w := httptest.NewRecorder()

	h.GetMaintenance(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}
