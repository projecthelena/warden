package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/projecthelena/warden/internal/config"
	"github.com/projecthelena/warden/internal/db"
)

var validHostPattern = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-.]*[a-zA-Z0-9])?(:\d{1,5})?$`)

const oidcRequestTimeout = 10 * time.Second

type SSOHandler struct {
	store  *db.Store
	config *config.Config
}

type oidcClaims struct {
	Subject       string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	PreferredName string `json:"preferred_username"`
	Picture       string `json:"picture"`
}

func NewSSOHandler(store *db.Store, cfg *config.Config) *SSOHandler {
	return &SSOHandler{store: store, config: cfg}
}

func oidcSubjectID(issuer, subject string) string {
	return url.QueryEscape(strings.TrimRight(issuer, "/")) + ":" + url.QueryEscape(subject)
}

func randomHex(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func (h *SSOHandler) absoluteOIDCRedirect(r *http.Request, path string) (string, error) {
	if !validHostPattern.MatchString(r.Host) {
		return "", fmt.Errorf("invalid host")
	}
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s%s", scheme, r.Host, path), nil
}

func emailDomainAllowed(email, allowedDomains string) bool {
	if strings.TrimSpace(allowedDomains) == "" {
		return true
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}
	for _, domain := range strings.Split(allowedDomains, ",") {
		if strings.EqualFold(strings.TrimSpace(domain), parts[1]) {
			return true
		}
	}
	return false
}
