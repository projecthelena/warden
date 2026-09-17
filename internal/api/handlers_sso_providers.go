package api

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-chi/chi/v5"
	"github.com/projecthelena/warden/internal/db"
)

var providerIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

type providerInput struct {
	Template       string `json:"template"`
	Name           string `json:"name"`
	IssuerURL      string `json:"issuerUrl"`
	ClientID       string `json:"clientId"`
	ClientSecret   string `json:"clientSecret"`
	AllowedDomains string `json:"allowedDomains"`
	AutoProvision  bool   `json:"autoProvision"`
	Enabled        bool   `json:"enabled"`
}

type providerResponse struct {
	ID               string `json:"id"`
	Template         string `json:"template"`
	Name             string `json:"name"`
	IssuerURL        string `json:"issuerUrl"`
	ClientID         string `json:"clientId"`
	SecretConfigured bool   `json:"secretConfigured"`
	AllowedDomains   string `json:"allowedDomains"`
	AutoProvision    bool   `json:"autoProvision"`
	Enabled          bool   `json:"enabled"`
	SortOrder        int    `json:"sortOrder"`
}

func providerDTO(provider db.SSOProvider) providerResponse {
	return providerResponse{
		ID:               provider.ID,
		Template:         provider.Template,
		Name:             provider.Name,
		IssuerURL:        provider.IssuerURL,
		ClientID:         provider.ClientID,
		SecretConfigured: provider.ClientSecret != "",
		AllowedDomains:   provider.AllowedDomains,
		AutoProvision:    provider.AutoProvision,
		Enabled:          provider.Enabled,
		SortOrder:        provider.SortOrder,
	}
}

func normalizeProvider(input providerInput) (providerInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.ClientID = strings.TrimSpace(input.ClientID)
	input.AllowedDomains = strings.TrimSpace(input.AllowedDomains)
	if len(input.Name) > 80 || len(input.IssuerURL) > 2048 || len(input.ClientID) > 512 || len(input.ClientSecret) > 4096 || len(input.AllowedDomains) > 2048 {
		return input, fmt.Errorf("provider configuration is too long")
	}
	if input.Template != "google" && input.Template != "oidc" {
		return input, fmt.Errorf("unsupported provider template")
	}
	if input.Template == "google" {
		input.IssuerURL = "https://accounts.google.com"
	}
	input.IssuerURL = strings.TrimRight(strings.TrimSpace(input.IssuerURL), "/")
	if input.Name == "" || input.IssuerURL == "" {
		return input, fmt.Errorf("name and issuer URL are required")
	}
	issuer, err := url.Parse(input.IssuerURL)
	if err != nil || issuer.Host == "" || (issuer.Scheme != "https" && issuer.Scheme != "http") || issuer.User != nil || issuer.RawQuery != "" || issuer.Fragment != "" {
		return input, fmt.Errorf("issuer URL must be HTTP(S) without credentials, query, or fragment")
	}
	for _, domain := range strings.Split(input.AllowedDomains, ",") {
		domain = strings.TrimSpace(domain)
		if domain != "" && (strings.ContainsAny(domain, "/:@") || !validHostPattern.MatchString(domain)) {
			return input, fmt.Errorf("invalid allowed domain %q", domain)
		}
	}
	if input.Enabled && (input.ClientID == "" || input.ClientSecret == "") {
		return input, fmt.Errorf("client ID and client secret are required when the provider is enabled")
	}
	return input, nil
}

func (h *SSOHandler) ListProviders(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, RoleAdmin) {
		return
	}
	providers, err := h.store.ListSSOProviders()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list identity providers")
		return
	}
	items := make([]providerResponse, 0, len(providers))
	for _, provider := range providers {
		items = append(items, providerDTO(provider))
	}
	writeJSON(w, http.StatusOK, map[string]any{"providers": items})
}

func (h *SSOHandler) PublicProviders(w http.ResponseWriter, r *http.Request) {
	providers, err := h.store.ListSSOProviders()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list identity providers")
		return
	}
	type item struct {
		ID       string `json:"id"`
		Template string `json:"template"`
		Name     string `json:"name"`
	}
	items := make([]item, 0, len(providers))
	for _, provider := range providers {
		if provider.Enabled && provider.ClientID != "" && provider.ClientSecret != "" && provider.IssuerURL != "" {
			items = append(items, item{ID: provider.ID, Template: provider.Template, Name: provider.Name})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"providers": items})
}

func (h *SSOHandler) CreateProvider(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, RoleAdmin) {
		return
	}
	var input providerInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	normalized, err := normalizeProvider(input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	idBytes := make([]byte, 8)
	if _, err := rand.Read(idBytes); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create identity provider")
		return
	}
	sortOrder, err := h.store.NextSSOProviderSortOrder()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create identity provider")
		return
	}
	provider := db.SSOProvider{
		ID:             "idp-" + hex.EncodeToString(idBytes),
		Template:       normalized.Template,
		Name:           normalized.Name,
		IssuerURL:      normalized.IssuerURL,
		ClientID:       normalized.ClientID,
		ClientSecret:   normalized.ClientSecret,
		AllowedDomains: normalized.AllowedDomains,
		AutoProvision:  normalized.AutoProvision,
		Enabled:        normalized.Enabled,
		SortOrder:      sortOrder,
	}
	if err := h.store.CreateSSOProvider(provider); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create identity provider")
		return
	}
	writeJSON(w, http.StatusCreated, providerDTO(provider))
}

func (h *SSOHandler) UpdateProvider(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, RoleAdmin) {
		return
	}
	id := chi.URLParam(r, "id")
	if !providerIDPattern.MatchString(id) {
		writeError(w, http.StatusBadRequest, "invalid provider ID")
		return
	}
	current, err := h.store.GetSSOProvider(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "identity provider not found")
		} else {
			writeError(w, http.StatusInternalServerError, "failed to load identity provider")
		}
		return
	}
	var input providerInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if input.ClientSecret == "" {
		input.ClientSecret = current.ClientSecret
	}
	normalized, err := normalizeProvider(input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	provider := db.SSOProvider{
		ID:             id,
		Template:       normalized.Template,
		Name:           normalized.Name,
		IssuerURL:      normalized.IssuerURL,
		ClientID:       normalized.ClientID,
		ClientSecret:   normalized.ClientSecret,
		AllowedDomains: normalized.AllowedDomains,
		AutoProvision:  normalized.AutoProvision,
		Enabled:        normalized.Enabled,
		SortOrder:      current.SortOrder,
		CreatedAt:      current.CreatedAt,
	}
	if err := h.store.UpdateSSOProvider(provider); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update identity provider")
		return
	}
	writeJSON(w, http.StatusOK, providerDTO(provider))
}

func (h *SSOHandler) DeleteProvider(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, RoleAdmin) {
		return
	}
	id := chi.URLParam(r, "id")
	if !providerIDPattern.MatchString(id) {
		writeError(w, http.StatusBadRequest, "invalid provider ID")
		return
	}
	err := h.store.DeleteSSOProvider(id)
	if errors.Is(err, db.ErrSSOProviderInUse) {
		writeError(w, http.StatusConflict, "disable this provider instead; users are linked to it")
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "identity provider not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete identity provider")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *SSOHandler) ReorderProviders(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, RoleAdmin) {
		return
	}
	var body struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.IDs) == 0 {
		writeError(w, http.StatusBadRequest, "provider IDs are required")
		return
	}
	providers, err := h.store.ListSSOProviders()
	if err != nil || len(body.IDs) != len(providers) {
		writeError(w, http.StatusBadRequest, "order must include every provider")
		return
	}
	seen := make(map[string]bool, len(body.IDs))
	for _, id := range body.IDs {
		if seen[id] || !providerIDPattern.MatchString(id) {
			writeError(w, http.StatusBadRequest, "invalid provider order")
			return
		}
		seen[id] = true
	}
	for _, provider := range providers {
		if !seen[provider.ID] {
			writeError(w, http.StatusBadRequest, "order must include every provider")
			return
		}
	}
	if err := h.store.ReorderSSOProviders(body.IDs); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to reorder identity providers")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *SSOHandler) TestProvider(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, RoleAdmin) {
		return
	}
	provider, err := h.store.GetSSOProvider(chi.URLParam(r, "id"))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "identity provider not found")
		} else {
			writeError(w, http.StatusInternalServerError, "failed to load identity provider")
		}
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), oidcRequestTimeout)
	defer cancel()
	if _, err := oidc.NewProvider(ctx, provider.IssuerURL); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"valid": false, "message": "OIDC discovery failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"valid": true, "message": "OIDC discovery succeeded"})
}
