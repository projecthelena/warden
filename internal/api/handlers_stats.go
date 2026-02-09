package api

import (
	"net/http"

	"github.com/projecthelena/warden/internal/db"
)

type StatsHandler struct {
	store *db.Store
}

func NewStatsHandler(store *db.Store) *StatsHandler {
	return &StatsHandler{store: store}
}

// GetStats returns system statistics including monitor counts and DB size.
// @Summary      Get system stats
// @Tags         stats
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object} object{version=string,dbSize=int,stats=db.SystemStats}
// @Failure      500  {string} string "Failed to get stats"
// @Router       /stats [get]
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
