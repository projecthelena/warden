package api

import (
	"net/http"

	"github.com/clusteruptime/clusteruptime/internal/db"
)

type StatsHandler struct {
	store *db.Store
}

func NewStatsHandler(store *db.Store) *StatsHandler {
	return &StatsHandler{store: store}
}

func (h *StatsHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.store.GetSystemStats()
	if err != nil {
		http.Error(w, "Failed to get stats", http.StatusInternalServerError)
		return
	}

	dbSize, err := h.store.GetDBSize()
	if err != nil {
		http.Error(w, "Failed to get db size", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"version": Version,
		"dbSize":  dbSize,
		"stats":   stats,
	}

	writeJSON(w, http.StatusOK, response)
}
