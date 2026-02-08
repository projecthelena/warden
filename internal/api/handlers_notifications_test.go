package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/projecthelena/warden/internal/db"
	"github.com/go-chi/chi/v5"
)

func newTestStore(t *testing.T) *db.Store {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("Failed to create test store: %v", err)
	}
	return store
}

func TestGetChannels(t *testing.T) {
	store := newTestStore(t)
	handler := NewNotificationChannelsHandler(store)

	channel := db.NotificationChannel{
		ID:        "nc1",
		Type:      "slack",
		Name:      "Test",
		Config:    "{}",
		Enabled:   true,
		CreatedAt: time.Now(),
	}
	if err := store.CreateNotificationChannel(channel); err != nil {
		t.Fatalf("Failed to create channel: %v", err)
	}

	req, _ := http.NewRequest("GET", "/api/notifications/channels", nil)
	rr := httptest.NewRecorder()

	handler.GetChannels(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}

	var response map[string][]db.NotificationChannel
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
	}

	if len(response["channels"]) != 1 {
		t.Errorf("expected 1 channel, got %d", len(response["channels"]))
	}
}

func TestCreateChannel(t *testing.T) {
	store := newTestStore(t)
	handler := NewNotificationChannelsHandler(store)

	payload := map[string]interface{}{
		"type":    "slack",
		"name":    "My Slack",
		"config":  map[string]string{"webhookUrl": "http://example.com"},
		"enabled": true,
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", "/api/notifications/channels", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	handler.CreateChannel(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusCreated)
	}

	channels, _ := store.GetNotificationChannels()
	if len(channels) != 1 {
		t.Errorf("expected 1 channel in db, got %d", len(channels))
	}
	if channels[0].Name != "My Slack" {
		t.Errorf("expected name 'My Slack', got '%s'", channels[0].Name)
	}
}

func TestDeleteChannel(t *testing.T) {
	store := newTestStore(t)
	handler := NewNotificationChannelsHandler(store)
	if err := store.CreateNotificationChannel(db.NotificationChannel{ID: "nc1", Type: "slack", Name: "To Delete", Config: "{}", Enabled: true}); err != nil {
		t.Fatalf("Failed to create channel: %v", err)
	}

	// Setup CHI router to handle params
	r := chi.NewRouter()
	r.Delete("/notifications/channels/{id}", handler.DeleteChannel)

	req, _ := http.NewRequest("DELETE", "/notifications/channels/nc1", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}

	channels, _ := store.GetNotificationChannels()
	if len(channels) != 0 {
		t.Errorf("expected 0 channels, got %d", len(channels))
	}
}
