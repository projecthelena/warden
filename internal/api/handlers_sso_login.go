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
	"github.com/go-chi/chi/v5"
	"github.com/projecthelena/warden/internal/db"
	"golang.org/x/oauth2"
)

func (h *SSOHandler) ProviderLogin(w http.ResponseWriter, r *http.Request) {
	providerID := chi.URLParam(r, "id")
	provider, _, oauthConfig, err := h.providerConfig(r.Context(), r, providerID)
	if err != nil {
		h.redirectSSOError(w, r, "sso_not_configured")
		return
	}

	state, err := randomHex(16)
	if err != nil {
		h.redirectSSOError(w, r, "internal_error")
		return
	}
	nonce, err := randomHex(16)
	if err != nil {
		h.redirectSSOError(w, r, "internal_error")
		return
	}
	verifier := oauth2.GenerateVerifier()
	h.setProviderCookie(w, "state", providerID, state, 300)
	h.setProviderCookie(w, "nonce", providerID, nonce, 300)
	h.setProviderCookie(w, "pkce", providerID, verifier, 300)

	options := []oauth2.AuthCodeOption{oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier)}
	if provider.Template == "google" {
		if domain := firstAllowedDomain(provider.AllowedDomains); domain != "" {
			options = append(options, oauth2.SetAuthURLParam("hd", domain))
		}
	}
	authorizationURL := oauthConfig.AuthCodeURL(state, options...)
	parsedURL, err := url.Parse(authorizationURL)
	if err != nil || parsedURL.Host == "" || (parsedURL.Scheme != "https" && parsedURL.Scheme != "http") || parsedURL.User != nil {
		h.redirectSSOError(w, r, "sso_not_configured")
		return
	}
	http.Redirect(w, r, authorizationURL, http.StatusTemporaryRedirect) // #nosec G710 -- validated OIDC endpoint from an admin-controlled issuer
}

func (h *SSOHandler) ProviderCallback(w http.ResponseWriter, r *http.Request) {
	providerID := chi.URLParam(r, "id")
	stateCookie, stateErr := r.Cookie(providerCookieName("state", providerID))
	nonceCookie, nonceErr := r.Cookie(providerCookieName("nonce", providerID))
	pkceCookie, pkceErr := r.Cookie(providerCookieName("pkce", providerID))
	h.clearProviderCookies(w, providerID)

	state := r.URL.Query().Get("state")
	if stateErr != nil || nonceErr != nil || pkceErr != nil || state == "" ||
		nonceCookie.Value == "" || pkceCookie.Value == "" ||
		subtle.ConstantTimeCompare([]byte(state), []byte(stateCookie.Value)) != 1 {
		h.redirectSSOError(w, r, "invalid_state")
		return
	}
	if r.URL.Query().Get("error") != "" {
		h.redirectSSOError(w, r, "oauth_denied")
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		h.redirectSSOError(w, r, "missing_code")
		return
	}

	provider, oidcProvider, oauthConfig, err := h.providerConfig(r.Context(), r, providerID)
	if err != nil {
		h.redirectSSOError(w, r, "sso_not_configured")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), oidcRequestTimeout)
	defer cancel()
	token, err := oauthConfig.Exchange(ctx, code, oauth2.VerifierOption(pkceCookie.Value))
	if err != nil {
		h.redirectSSOError(w, r, "token_exchange_failed")
		return
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		h.redirectSSOError(w, r, "invalid_token")
		return
	}
	idToken, err := oidcProvider.Verifier(&oidc.Config{ClientID: oauthConfig.ClientID}).Verify(ctx, rawIDToken)
	if err != nil || subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(nonceCookie.Value)) != 1 {
		h.redirectSSOError(w, r, "invalid_token")
		return
	}

	var claims oidcClaims
	if err := idToken.Claims(&claims); err != nil || claims.Subject == "" || claims.Email == "" || !claims.EmailVerified {
		h.redirectSSOError(w, r, "invalid_user_data")
		return
	}
	if !emailDomainAllowed(claims.Email, provider.AllowedDomains) {
		h.redirectSSOError(w, r, "domain_not_allowed")
		return
	}
	name := claims.Name
	if name == "" {
		name = claims.PreferredName
	}
	user, err := h.store.FindOrCreateSSOUser(
		provider.ID,
		oidcSubjectID(idToken.Issuer, claims.Subject),
		claims.Email,
		name,
		claims.Picture,
		provider.AutoProvision,
	)
	if err != nil {
		switch err {
		case db.ErrUserNotFound:
			h.redirectSSOError(w, r, "user_not_found")
		case db.ErrAccountLinkingNeed:
			h.redirectSSOError(w, r, "account_exists_link_required")
		default:
			log.Printf("AUDIT: [SSO] user creation failed: %v", err)
			h.redirectSSOError(w, r, "user_creation_failed")
		}
		return
	}
	h.createSSOSession(w, r, user)
}

func (h *SSOHandler) providerConfig(ctx context.Context, r *http.Request, id string) (*db.SSOProvider, *oidc.Provider, *oauth2.Config, error) {
	if !providerIDPattern.MatchString(id) {
		return nil, nil, nil, fmt.Errorf("invalid provider ID")
	}
	provider, err := h.store.GetSSOProvider(id)
	if err != nil || !provider.Enabled || provider.IssuerURL == "" || provider.ClientID == "" || provider.ClientSecret == "" {
		return nil, nil, nil, fmt.Errorf("provider is not configured")
	}
	ctx, cancel := context.WithTimeout(ctx, oidcRequestTimeout)
	defer cancel()
	oidcProvider, err := oidc.NewProvider(ctx, provider.IssuerURL)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("discover provider: %w", err)
	}
	redirectURL, err := h.absoluteOIDCRedirect(r, "/api/auth/sso/"+provider.ID+"/callback")
	if err != nil {
		return nil, nil, nil, err
	}
	config := &oauth2.Config{
		ClientID:     provider.ClientID,
		ClientSecret: provider.ClientSecret,
		RedirectURL:  redirectURL,
		Endpoint:     oidcProvider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}
	return provider, oidcProvider, config, nil
}

func (h *SSOHandler) createSSOSession(w http.ResponseWriter, r *http.Request, user *db.User) {
	sessionBytes := make([]byte, 32)
	if _, err := rand.Read(sessionBytes); err != nil {
		h.redirectSSOError(w, r, "session_error")
		return
	}
	sessionToken := hex.EncodeToString(sessionBytes)
	expiresAt := time.Now().Add(30 * 24 * time.Hour)
	if err := h.store.CreateSession(user.ID, sessionToken, expiresAt); err != nil {
		h.redirectSSOError(w, r, "session_error")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    sessionToken,
		Expires:  expiresAt,
		HttpOnly: true,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
		Secure:   h.config.CookieSecure,
	}) // #nosec G124 -- configurable for local HTTP development
	if user.Role == RoleStatusViewer {
		http.Redirect(w, r, "/my-pages", http.StatusTemporaryRedirect)
		return
	}
	http.Redirect(w, r, "/dashboard", http.StatusTemporaryRedirect)
}

func (h *SSOHandler) redirectSSOError(w http.ResponseWriter, r *http.Request, code string) {
	http.Redirect(w, r, "/login?error="+code, http.StatusTemporaryRedirect)
}

func firstAllowedDomain(domains string) string {
	for _, domain := range strings.Split(domains, ",") {
		if domain = strings.TrimSpace(domain); domain != "" {
			return domain
		}
	}
	return ""
}

func providerCookieName(kind, providerID string) string {
	return "oidc_" + kind + "_" + providerID
}

func (h *SSOHandler) setProviderCookie(w http.ResponseWriter, kind, providerID, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     providerCookieName(kind, providerID),
		Value:    value,
		MaxAge:   maxAge,
		HttpOnly: true,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
		Secure:   h.config.CookieSecure,
	}) // #nosec G124 -- configurable for local HTTP development
}

func (h *SSOHandler) clearProviderCookies(w http.ResponseWriter, providerID string) {
	for _, kind := range []string{"state", "nonce", "pkce"} {
		h.setProviderCookie(w, kind, providerID, "", -1)
	}
}
