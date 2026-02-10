package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/projecthelena/warden/internal/db"
	"github.com/projecthelena/warden/internal/uptime"
)

type SettingsHandler struct {
	store   *db.Store
	manager *uptime.Manager
}

func NewSettingsHandler(store *db.Store, manager *uptime.Manager) *SettingsHandler {
	return &SettingsHandler{store: store, manager: manager}
}

// GetSettings returns all application settings (secrets are masked).
// @Summary      Get settings
// @Tags         settings
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object} map[string]string
// @Router       /settings [get]
func (h *SettingsHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	// Latency Threshold
	val, err := h.store.GetSetting("latency_threshold")
	if err != nil {
		val = "1000"
	}

	// Data Retention
	retention, err := h.store.GetSetting("data_retention_days")
	if err != nil {
		retention = "30"
	}

	// Slack Notifications
	slackEnabled, _ := h.store.GetSetting("notifications.slack.enabled")
	slackWebhook, _ := h.store.GetSetting("notifications.slack.webhook_url")
	slackNotifyOn, _ := h.store.GetSetting("notifications.slack.notify_on")

	// SECURITY: Mask webhook URL to prevent exposure
	// Only show that it's configured, not the actual URL
	slackWebhookMasked := ""
	if slackWebhook != "" {
		if len(slackWebhook) > 30 {
			slackWebhookMasked = slackWebhook[:20] + "..." + slackWebhook[len(slackWebhook)-8:]
		} else {
			slackWebhookMasked = "***configured***"
		}
	}

	// SSO Settings (mask the secret)
	ssoGoogleEnabled, _ := h.store.GetSetting("sso.google.enabled")
	ssoGoogleClientID, _ := h.store.GetSetting("sso.google.client_id")
	ssoGoogleClientSecret, _ := h.store.GetSetting("sso.google.client_secret")
	ssoGoogleRedirectURL, _ := h.store.GetSetting("sso.google.redirect_url")
	ssoGoogleAllowedDomains, _ := h.store.GetSetting("sso.google.allowed_domains")
	ssoGoogleAutoProvision, _ := h.store.GetSetting("sso.google.auto_provision")

	// Only indicate if secret is configured, don't return actual value
	secretConfigured := "false"
	if ssoGoogleClientSecret != "" {
		secretConfigured = "true"
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"latency_threshold":                    val,
		"data_retention_days":                  retention,
		"notifications.slack.enabled":          slackEnabled,
		"notifications.slack.webhook_url":      slackWebhookMasked, // SECURITY: Masked for display
		"notifications.slack.webhook_configured": func() string { if slackWebhook != "" { return "true" }; return "false" }(),
		"notifications.slack.notify_on":        slackNotifyOn,
		"sso.google.enabled":                   ssoGoogleEnabled,
		"sso.google.client_id":                 ssoGoogleClientID,
		"sso.google.secret_configured":         secretConfigured,
		"sso.google.redirect_url":              ssoGoogleRedirectURL,
		"sso.google.allowed_domains":           ssoGoogleAllowedDomains,
		"sso.google.auto_provision":            ssoGoogleAutoProvision,
	})
}

// UpdateSettings patches application settings.
// @Summary      Update settings
// @Tags         settings
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body map[string]string true "Key-value pairs to update"
// @Success      200  {object} object{status=string}
// @Failure      400  {string} string "Invalid body"
// @Router       /settings [patch]
func (h *SettingsHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var body map[string]string
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid body", http.StatusBadRequest)
		return
	}

	if val, ok := body["latency_threshold"]; ok {
		// Validate int
		i, err := strconv.Atoi(val)
		if err != nil || i < 0 {
			http.Error(w, "Invalid latency_threshold", http.StatusBadRequest)
			return
		}

		if err := h.store.SetSetting("latency_threshold", val); err != nil {
			http.Error(w, "Failed to save latency_threshold", http.StatusInternalServerError)
			return
		}
		h.manager.SetLatencyThreshold(int64(i))
	}

	if val, ok := body["data_retention_days"]; ok {
		// Validate int
		i, err := strconv.Atoi(val)
		if err != nil || i < 1 {
			http.Error(w, "Invalid data_retention_days", http.StatusBadRequest)
			return
		}

		if err := h.store.SetSetting("data_retention_days", val); err != nil {
			http.Error(w, "Failed to save data_retention_days", http.StatusInternalServerError)
			return
		}
	}

	// Notifications Keys
	notificationKeys := []string{
		"notifications.slack.enabled",
		"notifications.slack.webhook_url",
		"notifications.slack.notify_on",
	}

	for _, key := range notificationKeys {
		if val, ok := body[key]; ok {
			if err := h.store.SetSetting(key, val); err != nil {
				http.Error(w, "Failed to save "+key, http.StatusInternalServerError)
				return
			}
		}
	}

	// SSO Settings Keys
	ssoKeys := []string{
		"sso.google.enabled",
		"sso.google.client_id",
		"sso.google.client_secret",
		"sso.google.redirect_url",
		"sso.google.allowed_domains",
		"sso.google.auto_provision",
	}

	for _, key := range ssoKeys {
		if val, ok := body[key]; ok {
			if err := h.store.SetSetting(key, val); err != nil {
				http.Error(w, "Failed to save "+key, http.StatusInternalServerError)
				return
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}
