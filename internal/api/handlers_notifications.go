package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/clusteruptime/clusteruptime/internal/db"
	"github.com/go-chi/chi/v5"
)

type NotificationChannelsHandler struct {
	store *db.Store
}

func NewNotificationChannelsHandler(store *db.Store) *NotificationChannelsHandler {
	return &NotificationChannelsHandler{store: store}
}

func (h *NotificationChannelsHandler) GetChannels(w http.ResponseWriter, r *http.Request) {
	channels, err := h.store.GetNotificationChannels()
	if err != nil {
		http.Error(w, "Failed to fetch channels", http.StatusInternalServerError)
		return
	}
	// Return as array directly to match frontend expectation or map?
	// Frontend expects { channels: [] } ? Actually frontend likely expects array or wrapper.
	// Store previously returned map for settings. Let's stick to wrapper.
	writeJSON(w, http.StatusOK, map[string]interface{}{"channels": channels})
}

func (h *NotificationChannelsHandler) CreateChannel(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Type    string                 `json:"type"`
		Name    string                 `json:"name"`
		Config  map[string]interface{} `json:"config"`
		Enabled bool                   `json:"enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid body", http.StatusBadRequest)
		return
	}

	if body.Type == "" || body.Name == "" {
		http.Error(w, "Type and Name are required", http.StatusBadRequest)
		return
	}

	configBytes, err := json.Marshal(body.Config)
	if err != nil {
		http.Error(w, "Invalid config", http.StatusBadRequest)
		return
	}

	// Generate ID
	id := "nc-" + generateRandomString(8)

	channel := db.NotificationChannel{
		ID:      id,
		Type:    body.Type,
		Name:    body.Name,
		Config:  string(configBytes),
		Enabled: body.Enabled,
	}

	if err := h.store.CreateNotificationChannel(channel); err != nil {
		http.Error(w, "Failed to create channel", http.StatusInternalServerError)
		return
	}

	// Return created channel with timestamp
	channel.CreatedAt = time.Now()
	writeJSON(w, http.StatusCreated, channel)
}

func (h *NotificationChannelsHandler) DeleteChannel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Missing ID", http.StatusBadRequest)
		return
	}

	if err := h.store.DeleteNotificationChannel(id); err != nil {
		http.Error(w, "Failed to delete channel", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func generateRandomString(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "rnd"
	}
	return hex.EncodeToString(b)
}
