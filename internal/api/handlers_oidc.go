package api

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/projecthelena/warden/internal/db"
	"golang.org/x/oauth2"
)

type oidcClaims struct {
	Subject       string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	PreferredName string `json:"preferred_username"`
	Picture       string `json:"picture"`
}

const oidcRequestTimeout = 10 * time.Second

func oidcSubjectID(issuer, subject string) string {
	return url.QueryEscape(strings.TrimRight(issuer, "/")) + ":" + url.QueryEscape(subject)
}

func (h *SSOHandler) oidcProvider(ctx context.Context) (*oidc.Provider, *oauth2.Config, error) {
	enabled, _ := h.store.GetSetting("sso.oidc.enabled")
	issuer, _ := h.store.GetSetting("sso.oidc.issuer_url")
	clientID, _ := h.store.GetSetting("sso.oidc.client_id")
	clientSecret, _ := h.store.GetSetting("sso.oidc.client_secret")
	redirectURL, _ := h.store.GetSetting("sso.oidc.redirect_url")
	if enabled != "true" || issuer == "" || clientID == "" || clientSecret == "" {
		return nil, nil, fmt.Errorf("OIDC is not configured")
	}
	if redirectURL == "" {
		redirectURL = "/api/auth/sso/oidc/callback"
	}
	provider, err := oidc.NewProvider(ctx, strings.TrimRight(issuer, "/"))
	if err != nil {
		return nil, nil, fmt.Errorf("discover OIDC provider: %w", err)
	}
	return provider, &oauth2.Config{ClientID: clientID, ClientSecret: clientSecret, RedirectURL: redirectURL, Endpoint: provider.Endpoint(), Scopes: []string{oidc.ScopeOpenID, "profile", "email"}}, nil
}

func (h *SSOHandler) absoluteOIDCRedirect(r *http.Request, redirectURL string) (string, error) {
	if !strings.HasPrefix(redirectURL, "/") {
		return redirectURL, nil
	}
	if !validHostPattern.MatchString(r.Host) {
		return "", fmt.Errorf("invalid host")
	}
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s%s", scheme, r.Host, redirectURL), nil
}

func (h *SSOHandler) OIDCLogin(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), oidcRequestTimeout)
	defer cancel()
	_, cfg, err := h.oidcProvider(ctx)
	if err != nil {
		http.Redirect(w, r, "/login?error=sso_not_configured", http.StatusTemporaryRedirect)
		return
	}
	if cfg.RedirectURL, err = h.absoluteOIDCRedirect(r, cfg.RedirectURL); err != nil {
		http.Redirect(w, r, "/login?error=invalid_request", http.StatusTemporaryRedirect)
		return
	}
	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		http.Redirect(w, r, "/login?error=internal_error", http.StatusTemporaryRedirect)
		return
	}
	state := hex.EncodeToString(stateBytes)
	http.SetCookie(w, &http.Cookie{Name: "oidc_state", Value: state, MaxAge: 300, HttpOnly: true, Path: "/", SameSite: http.SameSiteLaxMode, Secure: h.config.CookieSecure}) // #nosec G124 -- configurable for local HTTP dev
	http.Redirect(w, r, cfg.AuthCodeURL(state), http.StatusTemporaryRedirect)
}

func (h *SSOHandler) OIDCCallback(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie("oidc_state")
	state := r.URL.Query().Get("state")
	http.SetCookie(w, &http.Cookie{Name: "oidc_state", Value: "", MaxAge: -1, HttpOnly: true, Path: "/", SameSite: http.SameSiteLaxMode, Secure: h.config.CookieSecure}) // #nosec G124 -- configurable for local HTTP dev
	if err != nil || state == "" || subtle.ConstantTimeCompare([]byte(state), []byte(stateCookie.Value)) != 1 {
		http.Redirect(w, r, "/login?error=invalid_state", http.StatusTemporaryRedirect)
		return
	}
	if r.URL.Query().Get("error") != "" {
		http.Redirect(w, r, "/login?error=oauth_denied", http.StatusTemporaryRedirect)
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Redirect(w, r, "/login?error=missing_code", http.StatusTemporaryRedirect)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), oidcRequestTimeout)
	defer cancel()
	provider, cfg, err := h.oidcProvider(ctx)
	if err != nil {
		http.Redirect(w, r, "/login?error=sso_not_configured", http.StatusTemporaryRedirect)
		return
	}
	if cfg.RedirectURL, err = h.absoluteOIDCRedirect(r, cfg.RedirectURL); err != nil {
		http.Redirect(w, r, "/login?error=invalid_request", http.StatusTemporaryRedirect)
		return
	}
	token, err := cfg.Exchange(ctx, code)
	if err != nil {
		http.Redirect(w, r, "/login?error=token_exchange_failed", http.StatusTemporaryRedirect)
		return
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		http.Redirect(w, r, "/login?error=invalid_token", http.StatusTemporaryRedirect)
		return
	}
	idToken, err := provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}).Verify(ctx, rawIDToken)
	if err != nil {
		http.Redirect(w, r, "/login?error=invalid_token", http.StatusTemporaryRedirect)
		return
	}
	var claims oidcClaims
	if err := idToken.Claims(&claims); err != nil || claims.Subject == "" || claims.Email == "" || !claims.EmailVerified {
		http.Redirect(w, r, "/login?error=invalid_user_data", http.StatusTemporaryRedirect)
		return
	}
	allowedDomains, _ := h.store.GetSetting("sso.oidc.allowed_domains")
	if !emailDomainAllowed(claims.Email, allowedDomains) {
		http.Redirect(w, r, "/login?error=domain_not_allowed", http.StatusTemporaryRedirect)
		return
	}
	name := claims.Name
	if name == "" {
		name = claims.PreferredName
	}
	autoProvision, _ := h.store.GetSetting("sso.oidc.auto_provision")
	user, err := h.store.FindOrCreateSSOUser("oidc", oidcSubjectID(idToken.Issuer, claims.Subject), claims.Email, name, claims.Picture, autoProvision != "false")
	if err != nil {
		switch err {
		case db.ErrUserNotFound:
			http.Redirect(w, r, "/login?error=user_not_found", http.StatusTemporaryRedirect)
		case db.ErrAccountLinkingNeed:
			http.Redirect(w, r, "/login?error=account_exists_link_required", http.StatusTemporaryRedirect)
		default:
			log.Printf("AUDIT: [OIDC] user creation failed: %v", err)
			http.Redirect(w, r, "/login?error=user_creation_failed", http.StatusTemporaryRedirect)
		}
		return
	}
	sessionBytes := make([]byte, 32)
	if _, err := rand.Read(sessionBytes); err != nil {
		http.Redirect(w, r, "/login?error=session_error", http.StatusTemporaryRedirect)
		return
	}
	sessionToken := hex.EncodeToString(sessionBytes)
	expiresAt := time.Now().Add(30 * 24 * time.Hour)
	if err := h.store.CreateSession(user.ID, sessionToken, expiresAt); err != nil {
		http.Redirect(w, r, "/login?error=session_error", http.StatusTemporaryRedirect)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "auth_token", Value: sessionToken, Expires: expiresAt, HttpOnly: true, Path: "/", SameSite: http.SameSiteLaxMode, Secure: h.config.CookieSecure}) // #nosec G124 -- configurable for local HTTP dev
	if user.Role == RoleStatusViewer {
		http.Redirect(w, r, "/my-pages", http.StatusTemporaryRedirect)
	} else {
		http.Redirect(w, r, "/dashboard", http.StatusTemporaryRedirect)
	}
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
