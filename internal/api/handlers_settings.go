package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/clusteruptime/clusteruptime/internal/db"
	"github.com/clusteruptime/clusteruptime/internal/uptime"
)

type SettingsHandler struct {
	store   *db.Store
	manager *uptime.Manager
}

func NewSettingsHandler(store *db.Store, manager *uptime.Manager) *SettingsHandler {
	return &SettingsHandler{store: store, manager: manager}
}

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

	writeJSON(w, http.StatusOK, map[string]string{
		"latency_threshold":   val,
		"data_retention_days": retention,
	})
}

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
		// Manager reads this setting dynamically in retentionWorker,
		// but we could allow hot-reloading it if we exposed a setter?
		// For now, the worker reads it every run (daily), so it will pick it up next run.
		// If we want immediate effect, we'd need to trigger the worker.
		// But for retention, inevitable delay is fine.
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}
