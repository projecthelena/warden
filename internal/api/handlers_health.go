package api

import (
	"context"
	"net/http"
	"time"

	"github.com/projecthelena/warden/internal/db"
)

const pingTimeout = 5 * time.Second

// Healthz is the liveness probe — confirms the process is running.
func Healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"timestamp": time.Now().UTC(),
	})
}

// Readyz is the readiness probe — confirms the app can serve traffic.
func Readyz(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), pingTimeout)
		defer cancel()

		if err := store.PingContext(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{
				"status": "unavailable",
				"error":  "database not reachable",
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status":    "ok",
			"timestamp": time.Now().UTC(),
		})
	}
}
