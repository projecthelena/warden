package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/clusteruptime/clusteruptime/internal/config"
	"github.com/clusteruptime/clusteruptime/internal/db"
	"github.com/clusteruptime/clusteruptime/internal/uptime"
	"github.com/go-chi/chi/v5"
)

func setupTest(t *testing.T) (*CRUDHandler, *SettingsHandler, *AuthHandler, http.Handler, *db.Store) {
	store, _ := db.NewStore(":memory:")
	manager := uptime.NewManager(store)
	crudH := NewCRUDHandler(store, manager)
	settingsH := NewSettingsHandler(store, manager)

	cfg := config.Default()
	authH := NewAuthHandler(store, &cfg)

	// Create full router to test middleware if needed
	router := NewRouter(manager, store, &cfg)

	return crudH, settingsH, authH, router, store
}

func TestUpdateMonitor(t *testing.T) {
	crudH, _, _, _, s := setupTest(t)

	// Seed monitor
	if err := s.CreateMonitor(db.Monitor{ID: "m1", GroupID: "g-default", Name: "Old", URL: "http://old.com", Interval: 60}); err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}

	// Request Update
	payload := map[string]interface{}{
		"name":     "New",
		"url":      "http://new.com",
		"interval": 300,
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("PUT", "/api/monitors/m1", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Put("/api/monitors/{id}", crudH.UpdateMonitor)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify DB
	monitors, _ := s.GetMonitors()
	var m db.Monitor
	found := false
	for _, mon := range monitors {
		if mon.ID == "m1" {
			m = mon
			found = true
			break
		}
	}

	if !found {
		t.Fatal("Monitor m1 not found in DB")
	}

	if m.Name != "New" {
		t.Errorf("Name not updated, got %s", m.Name)
	}
	if m.Interval != 300 {
		t.Errorf("Interval not updated, got %d", m.Interval)
	}
}

func TestUpdateSettings(t *testing.T) {
	_, settingsH, _, _, s := setupTest(t)

	payload := map[string]string{
		"data_retention_days": "45",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("PATCH", "/api/settings", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	// Settings handler doesn't use URL params, so we can call directly or via router
	handler := http.HandlerFunc(settingsH.UpdateSettings)
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	// Verify DB
	val, err := s.GetSetting("data_retention_days")
	if err != nil {
		t.Fatalf("Failed to get setting: %v", err)
	}
	if val != "45" {
		t.Errorf("Expected 45, got %s", val)
	}
}
