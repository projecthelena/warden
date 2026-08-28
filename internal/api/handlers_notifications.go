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
		writeError(w, http.StatusInternalServerError, "Failed to fetch channels")
		return
	}
	// A webhook URL is a credential: anyone holding it can post into the channel. Only
	// the roles that can change it get to read it back.
	if !hasPermission(getUserRole(r), RoleEditor) {
		for i := range channels {
			channels[i].Config = maskSecrets(channels[i].Config)
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"channels": channels})
}

// maskSecrets blanks every value in a channel's JSON config, keeping the keys so the UI
// can still tell one channel from another.
//
// Everything is masked rather than a list of known credential names: a channel type
// added later would store its secret under a name that list does not have, and this
// would leak it. Masking everything fails closed instead.
func maskSecrets(config string) string {
	if config == "" {
		return config
	}
	var fields map[string]any
	if err := json.Unmarshal([]byte(config), &fields); err != nil {
		// Unparseable config could hold anything; say nothing rather than guess.
		return "{}"
	}

	for key, value := range fields {
		if v, ok := value.(string); ok && v != "" {
			fields[key] = "***"
		}
	}

	masked, err := json.Marshal(fields)
	if err != nil {
		return "{}"
	}
	return string(masked)
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
	if !requireRole(w, r, RoleEditor) {
		return
	}
	var body struct {
		Type    string                 `json:"type"`
		Name    string                 `json:"name"`
		Config  map[string]interface{} `json:"config"`
		Enabled bool                   `json:"enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid body")
		return
	}

	if body.Type == "" || body.Name == "" {
		writeError(w, http.StatusBadRequest, "Type and Name are required")
		return
	}

	// SECURITY: Validate name length
	if len(body.Name) > 255 {
		writeError(w, http.StatusBadRequest, "Name too long (max 255 characters)")
		return
	}

	// SECURITY: a channel is only stored once its config is known to be well formed
	//
	// Reported with writeError, like the rest of the API, rather than http.Error. The
	// dashboard reads a failure as JSON and shows data.error; a plain-text body makes
	// res.json() reject and every rejection collapses into "Failed to add channel." The
	// point of validating per field is telling the operator which field is wrong, and
	// that only survives if the reason arrives in the shape the client parses.
	if err := validateChannelConfig(body.Type, body.Config); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	configBytes, err := json.Marshal(body.Config)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid config")
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
		writeError(w, http.StatusInternalServerError, "Failed to create channel")
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
	if !requireRole(w, r, RoleEditor) {
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Missing ID")
		return
	}

	if err := h.store.DeleteNotificationChannel(id); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to delete channel")
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

// validateChannelConfig rejects a channel whose config cannot possibly work, before it
// reaches the database. Each type is checked on its own terms: a webhook needs a URL, an
// email channel needs a server and somewhere to send to. Types we do not know are left
// alone — the notifier reports the failure at send time.
func validateChannelConfig(channelType string, config map[string]interface{}) error {
	switch channelType {
	case "slack", "webhook":
		_, err := validateWebhookURL(extractWebhookURL(config))
		return err
	case "email":
		encoded, err := json.Marshal(config)
		if err != nil {
			return fmt.Errorf("invalid config")
		}
		// Parsed by the notifier itself so that the form and the send path agree on what
		// counts as valid, instead of drifting into two different sets of rules.
		return notifications.ValidateEmailConfig(string(encoded))
	default:
		return nil
	}
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
	if !requireRole(w, r, RoleEditor) {
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Missing ID")
		return
	}

	var body struct {
		Type    string                 `json:"type"`
		Name    string                 `json:"name"`
		Config  map[string]interface{} `json:"config"`
		Enabled *bool                  `json:"enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid body")
		return
	}

	if body.Type == "" || body.Name == "" {
		writeError(w, http.StatusBadRequest, "Type and Name are required")
		return
	}

	if len(body.Name) > 255 {
		writeError(w, http.StatusBadRequest, "Name too long (max 255 characters)")
		return
	}

	if err := validateChannelConfig(body.Type, body.Config); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	configBytes, err := json.Marshal(body.Config)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid config")
		return
	}

	channels, err := h.store.GetNotificationChannels()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to fetch channel")
		return
	}
	var current *db.NotificationChannel
	for i := range channels {
		if channels[i].ID == id {
			current = &channels[i]
			break
		}
	}
	if current == nil {
		writeError(w, http.StatusNotFound, "Channel not found")
		return
	}

	enabled := current.Enabled
	if body.Enabled != nil {
		enabled = *body.Enabled
	}

	if err := h.store.UpdateNotificationChannel(id, body.Name, body.Type, string(configBytes), enabled); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to update channel")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":      id,
		"type":    body.Type,
		"name":    body.Name,
		"config":  string(configBytes),
		"enabled": enabled,
	})
}

// TestChannel sends a test notification through the specified channel type and config.
func (h *NotificationChannelsHandler) TestChannel(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, RoleEditor) {
		return
	}
	var body struct {
		Type   string                 `json:"type"`
		Config map[string]interface{} `json:"config"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid body")
		return
	}

	if body.Type == "" {
		writeError(w, http.StatusBadRequest, "Type is required")
		return
	}

	if err := validateChannelConfig(body.Type, body.Config); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	configBytes, err := json.Marshal(body.Config)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid config")
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
