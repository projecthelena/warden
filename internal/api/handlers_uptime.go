package api

import (
	"encoding/json"
	"net/http"

	"github.com/clusteruptime/clusteruptime/internal/uptime"
)

type UptimeHandler struct {
	monitor *uptime.Monitor
}

func NewUptimeHandler(monitor *uptime.Monitor) *UptimeHandler {
	return &UptimeHandler{monitor: monitor}
}

func (h *UptimeHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	history := h.monitor.GetHistory()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
}
