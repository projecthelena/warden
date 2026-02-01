package api

import (
	"crypto/subtle"
	"log"
	"net/http"
	"strings"

	"github.com/clusteruptime/clusteruptime/internal/config"
	"github.com/clusteruptime/clusteruptime/internal/db"
	"github.com/clusteruptime/clusteruptime/internal/uptime"
)

type AdminHandler struct {
	store   *db.Store
	manager *uptime.Manager
	config  *config.Config
}

func NewAdminHandler(store *db.Store, manager *uptime.Manager, cfg *config.Config) *AdminHandler {
	return &AdminHandler{store: store, manager: manager, config: cfg}
}

// ResetDatabase performs a full database reset. REQUIRES ADMIN_SECRET.
// This is a destructive operation that should only be used for testing/development.
func (h *AdminHandler) ResetDatabase(w http.ResponseWriter, r *http.Request) {
	clientIP := extractIP(r)

	// SECURITY: Database reset ALWAYS requires ADMIN_SECRET
	// Regular session authentication is NOT sufficient for this destructive operation
	if h.config.AdminSecret == "" {
		log.Printf("AUDIT: [SECURITY] Database reset attempt from IP %s denied - ADMIN_SECRET not configured", clientIP)
		writeError(w, http.StatusForbidden, "admin operations not available")
		return
	}

	// Support both X-Admin-Secret header and Authorization: Bearer token
	secretHeader := r.Header.Get("X-Admin-Secret")
	authHeader := r.Header.Get("Authorization")
	bearerSecret := ""
	if strings.HasPrefix(authHeader, "Bearer ") {
		bearerSecret = strings.TrimPrefix(authHeader, "Bearer ")
	}

	// Use constant-time comparison to prevent timing attacks
	headerMatch := subtle.ConstantTimeCompare([]byte(secretHeader), []byte(h.config.AdminSecret)) == 1
	bearerMatch := bearerSecret != "" && subtle.ConstantTimeCompare([]byte(bearerSecret), []byte(h.config.AdminSecret)) == 1

	if !headerMatch && !bearerMatch {
		log.Printf("AUDIT: [SECURITY] Database reset attempt from IP %s denied - invalid admin secret", clientIP)
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	log.Printf("AUDIT: [ADMIN] Database reset initiated from IP %s", clientIP)
	h.performReset(w, clientIP)
}

func (h *AdminHandler) performReset(w http.ResponseWriter, clientIP string) {
	// Stop all monitoring before wiping DB to prevent FK violations
	h.manager.Reset()

	if err := h.store.Reset(); err != nil {
		log.Printf("AUDIT: [ADMIN] Database reset FAILED from IP %s: %v", clientIP, err)
		writeError(w, http.StatusInternalServerError, "operation failed")
		return
	}

	// Sync manager to start monitoring new seed data
	h.manager.Sync()

	log.Printf("AUDIT: [ADMIN] Database reset COMPLETED successfully from IP %s", clientIP)
	writeJSON(w, http.StatusOK, map[string]string{"message": "Database reset successfully"})
}
