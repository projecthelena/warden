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

// ResetDatabase performs a full database reset.
// This is a destructive operation that should only be used for testing/development.
// Accepts EITHER a valid session cookie (for frontend) OR admin secret (for E2E tests).
func (h *AdminHandler) ResetDatabase(w http.ResponseWriter, r *http.Request) {
	clientIP := extractIP(r)

	// Check 1: Session auth (for frontend button)
	if c, err := r.Cookie("auth_token"); err == nil {
		session, err := h.store.GetSession(c.Value)
		if err == nil && session != nil {
			log.Printf("AUDIT: [ADMIN] Database reset via session for user %d from IP %s", session.UserID, clientIP)
			h.performReset(w, clientIP)
			return
		}
	}

	// Check 2: Admin secret (for E2E tests / programmatic access)
	if h.config.AdminSecret != "" {
		secretHeader := r.Header.Get("X-Admin-Secret")
		authHeader := r.Header.Get("Authorization")
		bearerSecret := ""
		if strings.HasPrefix(authHeader, "Bearer ") {
			bearerSecret = strings.TrimPrefix(authHeader, "Bearer ")
		}

		headerMatch := subtle.ConstantTimeCompare([]byte(secretHeader), []byte(h.config.AdminSecret)) == 1
		bearerMatch := bearerSecret != "" && subtle.ConstantTimeCompare([]byte(bearerSecret), []byte(h.config.AdminSecret)) == 1

		if headerMatch || bearerMatch {
			log.Printf("AUDIT: [ADMIN] Database reset via admin secret from IP %s", clientIP)
			h.performReset(w, clientIP)
			return
		}
	}

	// Neither auth method succeeded
	log.Printf("AUDIT: [SECURITY] Database reset attempt from IP %s denied - no valid auth", clientIP)
	writeError(w, http.StatusUnauthorized, "unauthorized")
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
