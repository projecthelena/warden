package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/clusteruptime/clusteruptime/internal/config"
	"github.com/clusteruptime/clusteruptime/internal/db"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// validHostPattern validates host header format (hostname:port or hostname)
// SECURITY: Prevents Host header injection attacks in OAuth redirect URLs
var validHostPattern = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-\.]*[a-zA-Z0-9])?(:\d{1,5})?$`)

// maxUserInfoSize limits the size of the Google userinfo response to prevent memory exhaustion
const maxUserInfoSize = 1024 * 1024 // 1MB

type SSOHandler struct {
	store  *db.Store
	config *config.Config
}

func NewSSOHandler(store *db.Store, cfg *config.Config) *SSOHandler {
	return &SSOHandler{store: store, config: cfg}
}

// getGoogleOAuthConfig builds the OAuth2 config from stored settings
func (h *SSOHandler) getGoogleOAuthConfig() (*oauth2.Config, error) {
	clientID, err := h.store.GetSetting("sso.google.client_id")
	if err != nil || clientID == "" {
		return nil, fmt.Errorf("google oauth not configured: missing client_id")
	}

	clientSecret, err := h.store.GetSetting("sso.google.client_secret")
	if err != nil || clientSecret == "" {
		return nil, fmt.Errorf("google oauth not configured: missing client_secret")
	}

	// Check if SSO is enabled
	enabled, _ := h.store.GetSetting("sso.google.enabled")
	if enabled != "true" {
		return nil, fmt.Errorf("google sso is not enabled")
	}

	// Get redirect URL (optional override, default constructed from request)
	redirectURL, _ := h.store.GetSetting("sso.google.redirect_url")
	if redirectURL == "" {
		redirectURL = "/api/auth/sso/google/callback"
	}

	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{"email", "profile"},
		Endpoint:     google.Endpoint,
	}, nil
}

// GetSSOStatus returns the status of configured SSO providers (public endpoint)
func (h *SSOHandler) GetSSOStatus(w http.ResponseWriter, r *http.Request) {
	googleEnabled := false

	// Check if Google SSO is fully configured and enabled
	enabled, _ := h.store.GetSetting("sso.google.enabled")
	clientID, _ := h.store.GetSetting("sso.google.client_id")
	clientSecret, _ := h.store.GetSetting("sso.google.client_secret")

	if enabled == "true" && clientID != "" && clientSecret != "" {
		googleEnabled = true
	}

	writeJSON(w, http.StatusOK, map[string]bool{
		"google": googleEnabled,
	})
}

// GoogleLogin initiates the Google OAuth flow
func (h *SSOHandler) GoogleLogin(w http.ResponseWriter, r *http.Request) {
	oauthConfig, err := h.getGoogleOAuthConfig()
	if err != nil {
		http.Redirect(w, r, "/login?error=sso_not_configured", http.StatusTemporaryRedirect)
		return
	}

	// If redirect URL is relative, make it absolute
	if strings.HasPrefix(oauthConfig.RedirectURL, "/") {
		scheme := "http"
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}
		host := r.Host

		// SECURITY: Validate Host header to prevent header injection attacks
		if !validHostPattern.MatchString(host) {
			log.Printf("AUDIT: [SSO] Invalid Host header detected: %s", host)
			http.Redirect(w, r, "/login?error=invalid_request", http.StatusTemporaryRedirect)
			return
		}

		oauthConfig.RedirectURL = fmt.Sprintf("%s://%s%s", scheme, host, oauthConfig.RedirectURL)
	}

	// Generate state for CSRF protection
	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		http.Redirect(w, r, "/login?error=internal_error", http.StatusTemporaryRedirect)
		return
	}
	state := hex.EncodeToString(stateBytes)

	// Store state in a short-lived cookie
	// SECURITY: Use SameSite=Strict to prevent CSRF attacks on OAuth flow
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		MaxAge:   300, // 5 minutes
		HttpOnly: true,
		Path:     "/",
		SameSite: http.SameSiteStrictMode,
		Secure:   h.config.CookieSecure,
	})

	// Build OAuth URL options
	authOpts := []oauth2.AuthCodeOption{oauth2.AccessTypeOffline}

	// If allowed domains is configured, add the 'hd' (hosted domain) parameter
	// This tells Google to only show accounts from that domain in the account chooser
	allowedDomains, _ := h.store.GetSetting("sso.google.allowed_domains")
	if allowedDomains != "" {
		// Use the first domain if multiple are specified
		domains := strings.Split(allowedDomains, ",")
		if len(domains) > 0 {
			domain := strings.TrimSpace(domains[0])
			if domain != "" {
				authOpts = append(authOpts, oauth2.SetAuthURLParam("hd", domain))
			}
		}
	}

	// Redirect to Google's OAuth consent page
	url := oauthConfig.AuthCodeURL(state, authOpts...)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// clearStateCookie clears the OAuth state cookie
func (h *SSOHandler) clearStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    "",
		MaxAge:   -1,
		HttpOnly: true,
		Path:     "/",
		SameSite: http.SameSiteStrictMode,
		Secure:   h.config.CookieSecure,
	})
}

// GoogleCallback handles the OAuth callback from Google
func (h *SSOHandler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	// Verify state parameter
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil {
		h.clearStateCookie(w) // Always clear on error
		http.Redirect(w, r, "/login?error=invalid_state", http.StatusTemporaryRedirect)
		return
	}

	state := r.URL.Query().Get("state")
	// SECURITY: Use constant-time comparison to prevent timing attacks on state validation
	if state == "" || subtle.ConstantTimeCompare([]byte(state), []byte(stateCookie.Value)) != 1 {
		h.clearStateCookie(w)
		http.Redirect(w, r, "/login?error=invalid_state", http.StatusTemporaryRedirect)
		return
	}

	// Clear state cookie immediately after validation
	h.clearStateCookie(w)

	// Check for error from Google
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		http.Redirect(w, r, "/login?error=oauth_denied", http.StatusTemporaryRedirect)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Redirect(w, r, "/login?error=missing_code", http.StatusTemporaryRedirect)
		return
	}

	oauthConfig, err := h.getGoogleOAuthConfig()
	if err != nil {
		http.Redirect(w, r, "/login?error=sso_not_configured", http.StatusTemporaryRedirect)
		return
	}

	// If redirect URL is relative, make it absolute
	if strings.HasPrefix(oauthConfig.RedirectURL, "/") {
		scheme := "http"
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}
		host := r.Host

		// SECURITY: Validate Host header to prevent header injection attacks
		if !validHostPattern.MatchString(host) {
			log.Printf("AUDIT: [SSO] Invalid Host header detected in callback: %s", host)
			http.Redirect(w, r, "/login?error=invalid_request", http.StatusTemporaryRedirect)
			return
		}

		oauthConfig.RedirectURL = fmt.Sprintf("%s://%s%s", scheme, host, oauthConfig.RedirectURL)
	}

	// Exchange code for token
	token, err := oauthConfig.Exchange(r.Context(), code)
	if err != nil {
		http.Redirect(w, r, "/login?error=token_exchange_failed", http.StatusTemporaryRedirect)
		return
	}

	// SECURITY: Validate the OAuth token response
	// Check that we received a valid access token
	if !token.Valid() {
		log.Printf("AUDIT: [SSO] Invalid OAuth token received from Google")
		http.Redirect(w, r, "/login?error=invalid_token", http.StatusTemporaryRedirect)
		return
	}

	// Verify token type is Bearer (standard OAuth2 access token type)
	if token.TokenType != "" && token.TokenType != "Bearer" {
		log.Printf("AUDIT: [SSO] Unexpected token type from Google: %s", token.TokenType)
		http.Redirect(w, r, "/login?error=invalid_token_type", http.StatusTemporaryRedirect)
		return
	}

	// Get user info from Google
	client := oauthConfig.Client(r.Context(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		http.Redirect(w, r, "/login?error=userinfo_failed", http.StatusTemporaryRedirect)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	// Limit response size to prevent memory exhaustion
	limitedReader := io.LimitReader(resp.Body, maxUserInfoSize)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		http.Redirect(w, r, "/login?error=userinfo_read_failed", http.StatusTemporaryRedirect)
		return
	}

	var googleUser struct {
		ID            string `json:"id"`
		Email         string `json:"email"`
		VerifiedEmail bool   `json:"verified_email"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	if err := json.Unmarshal(body, &googleUser); err != nil {
		http.Redirect(w, r, "/login?error=userinfo_parse_failed", http.StatusTemporaryRedirect)
		return
	}

	// Validate required fields
	if googleUser.Email == "" || googleUser.ID == "" {
		http.Redirect(w, r, "/login?error=invalid_user_data", http.StatusTemporaryRedirect)
		return
	}

	// SECURITY: Require verified email to prevent account hijacking
	if !googleUser.VerifiedEmail {
		http.Redirect(w, r, "/login?error=email_not_verified", http.StatusTemporaryRedirect)
		return
	}

	// Check domain restriction
	allowedDomains, _ := h.store.GetSetting("sso.google.allowed_domains")
	if allowedDomains != "" {
		// Safely extract domain from email
		emailParts := strings.Split(googleUser.Email, "@")
		if len(emailParts) != 2 || emailParts[1] == "" {
			http.Redirect(w, r, "/login?error=invalid_user_data", http.StatusTemporaryRedirect)
			return
		}
		emailDomain := strings.ToLower(emailParts[1])

		domains := strings.Split(allowedDomains, ",")
		domainAllowed := false
		for _, d := range domains {
			if strings.TrimSpace(strings.ToLower(d)) == emailDomain {
				domainAllowed = true
				break
			}
		}
		if !domainAllowed {
			http.Redirect(w, r, "/login?error=domain_not_allowed", http.StatusTemporaryRedirect)
			return
		}
	}

	// Check auto-provision setting BEFORE attempting to find/create user
	autoProvision, _ := h.store.GetSetting("sso.google.auto_provision")

	// Find or create user
	clientIP := extractIP(r)
	user, err := h.store.FindOrCreateSSOUser("google", googleUser.ID, googleUser.Email, googleUser.Name, googleUser.Picture, autoProvision != "false")
	if err != nil {
		if err == db.ErrUserNotFound {
			log.Printf("AUDIT: [SSO] Google login denied - user not found for email %s from IP %s", googleUser.Email, clientIP)
			http.Redirect(w, r, "/login?error=user_not_found", http.StatusTemporaryRedirect)
			return
		}
		if err == db.ErrAccountLinkingNeed {
			// Account exists with password - user must link SSO through settings
			log.Printf("AUDIT: [SSO] Google login denied - account linking required for email %s from IP %s", googleUser.Email, clientIP)
			http.Redirect(w, r, "/login?error=account_exists_link_required", http.StatusTemporaryRedirect)
			return
		}
		log.Printf("AUDIT: [SSO] Google login failed - user creation error for email %s from IP %s: %v", googleUser.Email, clientIP, err)
		http.Redirect(w, r, "/login?error=user_creation_failed", http.StatusTemporaryRedirect)
		return
	}
	log.Printf("AUDIT: [SSO] Successful Google login for user '%s' (ID: %d, email: %s) from IP %s", user.Username, user.ID, googleUser.Email, clientIP)

	// Create session (same as regular login)
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		http.Redirect(w, r, "/login?error=session_error", http.StatusTemporaryRedirect)
		return
	}
	sessionToken := hex.EncodeToString(tokenBytes)
	expiresAt := time.Now().Add(30 * 24 * time.Hour)

	if err := h.store.CreateSession(user.ID, sessionToken, expiresAt); err != nil {
		http.Redirect(w, r, "/login?error=session_error", http.StatusTemporaryRedirect)
		return
	}

	// Set auth cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    sessionToken,
		Expires:  expiresAt,
		HttpOnly: true,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
		Secure:   h.config.CookieSecure,
	})

	// Redirect to dashboard
	http.Redirect(w, r, "/dashboard", http.StatusTemporaryRedirect)
}

// TestSSOConfig tests if the SSO configuration is valid (admin only)
func (h *SSOHandler) TestSSOConfig(w http.ResponseWriter, r *http.Request) {
	clientID, _ := h.store.GetSetting("sso.google.client_id")
	clientSecret, _ := h.store.GetSetting("sso.google.client_secret")

	if clientID == "" || clientSecret == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"valid":   false,
			"message": "Client ID and Client Secret are required",
		})
		return
	}

	// Basic validation - check that credentials look valid (not empty, reasonable length)
	if len(clientID) < 20 {
		writeJSON(w, http.StatusOK, map[string]any{
			"valid":   false,
			"message": "Client ID appears to be invalid (too short)",
		})
		return
	}

	if len(clientSecret) < 10 {
		writeJSON(w, http.StatusOK, map[string]any{
			"valid":   false,
			"message": "Client Secret appears to be invalid (too short)",
		})
		return
	}

	// We can't fully validate without attempting auth, so just confirm format
	writeJSON(w, http.StatusOK, map[string]any{
		"valid":   true,
		"message": "Configuration looks valid. Enable SSO and test the login flow to verify.",
	})
}
