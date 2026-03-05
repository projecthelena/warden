package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/projecthelena/warden/internal/db"
	"github.com/projecthelena/warden/internal/notifications"
)

type NotificationChannelsHandler struct {
	store *db.Store
}

func NewNotificationChannelsHandler(store *db.Store) *NotificationChannelsHandler {
	return &NotificationChannelsHandler{store: store}
}

// GetChannels returns all configured notification channels.
// @Summary      List notification channels
// @Tags         notifications
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object} object{channels=[]db.NotificationChannel}
// @Failure      500  {string} string "Failed to fetch channels"
// @Router       /notifications/channels [get]
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

// CreateChannel adds a new notification channel (e.g. Slack webhook).
// @Summary      Create notification channel
// @Tags         notifications
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{type=string,name=string,config=object,enabled=bool} true "Channel config"
// @Success      201  {object} db.NotificationChannel
// @Failure      400  {string} string "Type and Name are required"
// @Router       /notifications/channels [post]
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

	// SECURITY: Validate name length
	if len(body.Name) > 255 {
		http.Error(w, "Name too long (max 255 characters)", http.StatusBadRequest)
		return
	}

	// SECURITY: Validate webhook URL for channel types that use it
	if body.Type == "slack" || body.Type == "webhook" {
		webhookURL := extractWebhookURL(body.Config)
		if _, err := validateWebhookURL(webhookURL); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
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

// DeleteChannel removes a notification channel.
// @Summary      Delete notification channel
// @Tags         notifications
// @Produce      json
// @Security     BearerAuth
// @Param        id   path string true "Channel ID"
// @Success      200  "OK"
// @Failure      400  {string} string "Missing ID"
// @Router       /notifications/channels/{id} [delete]
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

// validateWebhookURL checks that a URL is valid HTTP(S) and within length limits.
func validateWebhookURL(rawURL string) (string, error) {
	if rawURL == "" {
		return "", fmt.Errorf("webhook URL is required")
	}
	if len(rawURL) > 2048 {
		return "", fmt.Errorf("webhook URL too long (max 2048 characters)")
	}
	parsedURL, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid webhook URL format")
	}
	if parsedURL.Scheme != "https" && parsedURL.Scheme != "http" {
		return "", fmt.Errorf("webhook URL must use HTTP or HTTPS")
	}
	return rawURL, nil
}

// extractWebhookURL pulls the webhook URL from a config map, supporting both key names.
func extractWebhookURL(config map[string]interface{}) string {
	if u, ok := config["webhook_url"].(string); ok {
		return u
	}
	if u, ok := config["webhookUrl"].(string); ok {
		return u
	}
	return ""
}

// UpdateChannel modifies an existing notification channel.
func (h *NotificationChannelsHandler) UpdateChannel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Missing ID", http.StatusBadRequest)
		return
	}

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

	if len(body.Name) > 255 {
		http.Error(w, "Name too long (max 255 characters)", http.StatusBadRequest)
		return
	}

	// Validate webhook URL for types that use it
	if body.Type == "slack" || body.Type == "webhook" {
		webhookURL := extractWebhookURL(body.Config)
		if _, err := validateWebhookURL(webhookURL); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	configBytes, err := json.Marshal(body.Config)
	if err != nil {
		http.Error(w, "Invalid config", http.StatusBadRequest)
		return
	}

	if err := h.store.UpdateNotificationChannel(id, body.Name, body.Type, string(configBytes), body.Enabled); err != nil {
		http.Error(w, "Failed to update channel", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":      id,
		"type":    body.Type,
		"name":    body.Name,
		"config":  string(configBytes),
		"enabled": body.Enabled,
	})
}

// TestChannel sends a test notification through the specified channel type and config.
func (h *NotificationChannelsHandler) TestChannel(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Type   string                 `json:"type"`
		Config map[string]interface{} `json:"config"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid body", http.StatusBadRequest)
		return
	}

	if body.Type == "" {
		http.Error(w, "Type is required", http.StatusBadRequest)
		return
	}

	// Validate webhook URL
	webhookURL := extractWebhookURL(body.Config)
	if _, err := validateWebhookURL(webhookURL); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	configBytes, err := json.Marshal(body.Config)
	if err != nil {
		http.Error(w, "Invalid config", http.StatusBadRequest)
		return
	}

	testEvent := notifications.NotificationEvent{
		MonitorID:   "test-monitor-001",
		MonitorName: "Example Monitor",
		MonitorURL:  "https://example.com",
		Type:        notifications.EventDown,
		Message:     "This is a test notification from Warden.",
		Time:        time.Now(),
	}

	if err := notifications.SendDirect(body.Type, string(configBytes), testEvent); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "Test failed: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "message": "Test notification sent successfully"})
}

func generateRandomString(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "rnd"
	}
	return hex.EncodeToString(b)
}
