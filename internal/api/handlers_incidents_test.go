package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/clusteruptime/clusteruptime/internal/db"
)

func TestIncidentHandler(t *testing.T) {
	s, _ := db.NewStore(":memory:")
	h := NewIncidentHandler(s)

	// Create Incident
	payload := map[string]string{
		"title":       "Database Down",
		"description": "Investigating",
		"severity":    "critical",
		"status":      "investigating",
		"startTime":   "2023-10-27T10:00:00Z",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/incidents", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	h.CreateIncident(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("CreateIncident failed: %d", w.Code)
	}

	// List Incidents
	req = httptest.NewRequest("GET", "/api/incidents", nil)
	w = httptest.NewRecorder()
	h.GetIncidents(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("GetIncidents failed: %d", w.Code)
	}
}
