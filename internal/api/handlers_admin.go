package api

import (
	"log"
	"net/http"

	"github.com/clusteruptime/clusteruptime/internal/db"
	"github.com/clusteruptime/clusteruptime/internal/uptime"
)

type AdminHandler struct {
	store   *db.Store
	manager *uptime.Manager
}

func NewAdminHandler(store *db.Store, manager *uptime.Manager) *AdminHandler {
	return &AdminHandler{store: store, manager: manager}
}

const TestResetKey = "clusteruptime-e2e-magic-key"
const TestResetHeader = "X-Cluster-Test-Key"

func (h *AdminHandler) ResetDatabase(w http.ResponseWriter, r *http.Request) {
	// 1. Check Test Key (Bypass Auth)
	if r.Header.Get(TestResetHeader) == TestResetKey {
		log.Println("ADMIN: Resetting database via Test Key bypass.")
		h.performReset(w)
		return
	}

	// 2. Check Standard Auth (Cookie)
	c, err := r.Cookie("auth_token")
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	sess, err := h.store.GetSession(c.Value)
	if err != nil || sess == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// (Optional: Check if user is admin? For now system is single-tenant or all-admin)

	log.Println("ADMIN: Initiating full database reset requested by user", sess.UserID)
	h.performReset(w)
}

func (h *AdminHandler) performReset(w http.ResponseWriter) {
	// Stop all monitoring before wiping DB to prevent FK violations
	h.manager.Reset()

	if err := h.store.Reset(); err != nil {
		log.Printf("Failed to reset database: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to reset database")
		return
	}

	// Sync manager to start monitoring new seed data
	h.manager.Sync()

	log.Println("ADMIN: Database reset successful.")
	writeJSON(w, http.StatusOK, map[string]string{"message": "Database reset successfully"})
}
