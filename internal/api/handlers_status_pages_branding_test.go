package api

// Branding-specific tests for the status page configuration handler.
// Covers favicon, logo/favicon clearing, preserve-when-absent, and GetAll branding fields.
// The helpers (newStatusPageTestEnv, seedPage, makeRequest, decodeJSON, etc.) live in
// handlers_status_pages_test.go and are shared across the package's test files.

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

const testDataURI = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="

// ============================================================
// Favicon URL tests
// ============================================================

func TestBranding_SetFaviconURL(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)
	seedPage(t, store, "favicon-set", "Favicon Set", nil, true, true)

	payload := map[string]interface{}{
		"public":     true,
		"enabled":    true,
		"title":      "Favicon Set",
		"faviconUrl": "https://example.com/favicon.ico",
	}

	w := httptest.NewRecorder()
	spH.Toggle(w, makeRequest("PATCH", "/api/status-pages/favicon-set", "favicon-set", payload))

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	page, _ := store.GetStatusPageBySlug("favicon-set")
	if page == nil {
		t.Fatal("Page not found after save")
	}
	if page.FaviconURL != "https://example.com/favicon.ico" {
		t.Errorf("Expected faviconUrl 'https://example.com/favicon.ico', got '%s'", page.FaviconURL)
	}
}

func TestBranding_DataURIFaviconAccepted(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)
	seedPage(t, store, "favicon-data", "Favicon Data", nil, true, true)

	payload := map[string]interface{}{
		"public":     true,
		"enabled":    true,
		"title":      "Favicon Data",
		"faviconUrl": testDataURI,
	}

	w := httptest.NewRecorder()
	spH.Toggle(w, makeRequest("PATCH", "/api/status-pages/favicon-data", "favicon-data", payload))

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 for data URI favicon, got %d (body: %s)", w.Code, w.Body.String())
	}

	page, _ := store.GetStatusPageBySlug("favicon-data")
	if page.FaviconURL != testDataURI {
		t.Error("Data URI favicon should be stored verbatim")
	}
}

func TestBranding_InvalidFaviconURLRejected(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)
	seedPage(t, store, "favicon-bad", "Favicon Bad", nil, true, true)

	badURLs := []string{
		"ftp://invalid-protocol.com/favicon.ico",
		"javascript:alert(1)",
		"file:///etc/passwd",
		"not-a-url-at-all",
	}

	for _, url := range badURLs {
		payload := map[string]interface{}{
			"public":     true,
			"enabled":    true,
			"title":      "Favicon Bad",
			"faviconUrl": url,
		}

		w := httptest.NewRecorder()
		spH.Toggle(w, makeRequest("PATCH", "/api/status-pages/favicon-bad", "favicon-bad", payload))

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 for invalid favicon URL %q, got %d", url, w.Code)
		}
	}
}

// ============================================================
// Clearing tests (regression for the "|| undefined" frontend bug)
// These verify the backend correctly handles "" (explicit clear) vs nil (absent/preserve).
// ============================================================

// TestBranding_ClearLogoURL_WithEmptyString verifies that sending an empty string for
// logoUrl clears a previously-set logo. This is the backend half of the fix: the
// frontend used to send undefined (omit) instead of "" when the user cleared the field,
// which caused the backend to preserve the old value.
func TestBranding_ClearLogoURL_WithEmptyString(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)
	seedPage(t, store, "logo-clear", "Logo Clear", nil, true, true)

	// First, set a logo URL.
	setPayload := map[string]interface{}{
		"public":  true,
		"enabled": true,
		"title":   "Logo Clear",
		"logoUrl": "https://example.com/logo.png",
	}
	w := httptest.NewRecorder()
	spH.Toggle(w, makeRequest("PATCH", "/api/status-pages/logo-clear", "logo-clear", setPayload))
	if w.Code != http.StatusOK {
		t.Fatalf("Set failed: %d %s", w.Code, w.Body.String())
	}

	page, _ := store.GetStatusPageBySlug("logo-clear")
	if page.LogoURL != "https://example.com/logo.png" {
		t.Fatal("Logo not set, cannot test clearing")
	}

	// Now clear it by sending an explicit empty string.
	clearPayload := map[string]interface{}{
		"public":  true,
		"enabled": true,
		"title":   "Logo Clear",
		"logoUrl": "", // explicit empty string — must clear, not preserve
	}
	w = httptest.NewRecorder()
	spH.Toggle(w, makeRequest("PATCH", "/api/status-pages/logo-clear", "logo-clear", clearPayload))
	if w.Code != http.StatusOK {
		t.Fatalf("Clear failed: %d %s", w.Code, w.Body.String())
	}

	page, _ = store.GetStatusPageBySlug("logo-clear")
	if page.LogoURL != "" {
		t.Errorf("Expected logoUrl to be cleared (empty), got '%s'", page.LogoURL)
	}
}

func TestBranding_ClearFaviconURL_WithEmptyString(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)
	seedPage(t, store, "fav-clear", "Fav Clear", nil, true, true)

	// First, set a favicon URL.
	setPayload := map[string]interface{}{
		"public":     true,
		"enabled":    true,
		"title":      "Fav Clear",
		"faviconUrl": "https://example.com/favicon.ico",
	}
	w := httptest.NewRecorder()
	spH.Toggle(w, makeRequest("PATCH", "/api/status-pages/fav-clear", "fav-clear", setPayload))
	if w.Code != http.StatusOK {
		t.Fatalf("Set failed: %d", w.Code)
	}

	page, _ := store.GetStatusPageBySlug("fav-clear")
	if page.FaviconURL != "https://example.com/favicon.ico" {
		t.Fatal("Favicon not set, cannot test clearing")
	}

	// Clear by sending explicit empty string.
	clearPayload := map[string]interface{}{
		"public":     true,
		"enabled":    true,
		"title":      "Fav Clear",
		"faviconUrl": "",
	}
	w = httptest.NewRecorder()
	spH.Toggle(w, makeRequest("PATCH", "/api/status-pages/fav-clear", "fav-clear", clearPayload))
	if w.Code != http.StatusOK {
		t.Fatalf("Clear failed: %d", w.Code)
	}

	page, _ = store.GetStatusPageBySlug("fav-clear")
	if page.FaviconURL != "" {
		t.Errorf("Expected faviconUrl to be cleared (empty), got '%s'", page.FaviconURL)
	}
}

func TestBranding_ClearAccentColor_WithEmptyString(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)
	seedPage(t, store, "color-clear", "Color Clear", nil, true, true)

	// Set accent color first.
	setPayload := map[string]interface{}{
		"public":      true,
		"enabled":     true,
		"title":       "Color Clear",
		"accentColor": "#FF5500",
	}
	w := httptest.NewRecorder()
	spH.Toggle(w, makeRequest("PATCH", "/api/status-pages/color-clear", "color-clear", setPayload))
	if w.Code != http.StatusOK {
		t.Fatalf("Set failed: %d", w.Code)
	}

	page, _ := store.GetStatusPageBySlug("color-clear")
	if page.AccentColor != "#FF5500" {
		t.Fatal("Accent color not set, cannot test clearing")
	}

	// Clear by sending explicit empty string.
	clearPayload := map[string]interface{}{
		"public":      true,
		"enabled":     true,
		"title":       "Color Clear",
		"accentColor": "",
	}
	w = httptest.NewRecorder()
	spH.Toggle(w, makeRequest("PATCH", "/api/status-pages/color-clear", "color-clear", clearPayload))
	if w.Code != http.StatusOK {
		t.Fatalf("Clear failed: %d", w.Code)
	}

	page, _ = store.GetStatusPageBySlug("color-clear")
	if page.AccentColor != "" {
		t.Errorf("Expected accentColor to be cleared, got '%s'", page.AccentColor)
	}
}

// ============================================================
// Preserve-when-absent tests
// Sending a PATCH without logoUrl/faviconUrl should preserve the existing values.
// This is the "only change what was explicitly sent" semantics.
// ============================================================

func TestBranding_PreserveLogoURL_WhenFieldAbsent(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)
	seedPage(t, store, "logo-preserve", "Logo Preserve", nil, true, true)

	// Set a logo URL first.
	setPayload := map[string]interface{}{
		"public":  true,
		"enabled": true,
		"title":   "Logo Preserve",
		"logoUrl": "https://example.com/preserve.png",
	}
	w := httptest.NewRecorder()
	spH.Toggle(w, makeRequest("PATCH", "/api/status-pages/logo-preserve", "logo-preserve", setPayload))
	if w.Code != http.StatusOK {
		t.Fatalf("Set failed: %d", w.Code)
	}

	// Update something else without including logoUrl in the payload.
	updatePayload := map[string]interface{}{
		"public":  false,
		"enabled": true,
		"title":   "Logo Preserve Updated",
		// logoUrl intentionally omitted — should be preserved
	}
	w = httptest.NewRecorder()
	spH.Toggle(w, makeRequest("PATCH", "/api/status-pages/logo-preserve", "logo-preserve", updatePayload))
	if w.Code != http.StatusOK {
		t.Fatalf("Update failed: %d", w.Code)
	}

	page, _ := store.GetStatusPageBySlug("logo-preserve")
	if page.LogoURL != "https://example.com/preserve.png" {
		t.Errorf("Expected logoUrl to be preserved, got '%s'", page.LogoURL)
	}
	if page.Title != "Logo Preserve Updated" {
		t.Errorf("Expected title to be updated, got '%s'", page.Title)
	}
}

func TestBranding_PreserveFaviconURL_WhenFieldAbsent(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)
	seedPage(t, store, "fav-preserve", "Fav Preserve", nil, true, true)

	// Set a favicon URL.
	setPayload := map[string]interface{}{
		"public":     true,
		"enabled":    true,
		"title":      "Fav Preserve",
		"faviconUrl": "https://example.com/fav-preserve.ico",
	}
	w := httptest.NewRecorder()
	spH.Toggle(w, makeRequest("PATCH", "/api/status-pages/fav-preserve", "fav-preserve", setPayload))
	if w.Code != http.StatusOK {
		t.Fatalf("Set failed: %d", w.Code)
	}

	// Toggle enabled without providing faviconUrl.
	updatePayload := map[string]interface{}{
		"public":  true,
		"enabled": false,
		"title":   "Fav Preserve",
		// faviconUrl intentionally omitted
	}
	w = httptest.NewRecorder()
	spH.Toggle(w, makeRequest("PATCH", "/api/status-pages/fav-preserve", "fav-preserve", updatePayload))
	if w.Code != http.StatusOK {
		t.Fatalf("Update failed: %d", w.Code)
	}

	page, _ := store.GetStatusPageBySlug("fav-preserve")
	if page.FaviconURL != "https://example.com/fav-preserve.ico" {
		t.Errorf("Expected faviconUrl to be preserved after toggle, got '%s'", page.FaviconURL)
	}
}

func TestBranding_PreserveAccentColor_WhenFieldAbsent(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)
	seedPage(t, store, "color-preserve", "Color Preserve", nil, true, true)

	setPayload := map[string]interface{}{
		"public":      true,
		"enabled":     true,
		"title":       "Color Preserve",
		"accentColor": "#AABBCC",
	}
	w := httptest.NewRecorder()
	spH.Toggle(w, makeRequest("PATCH", "/api/status-pages/color-preserve", "color-preserve", setPayload))
	if w.Code != http.StatusOK {
		t.Fatalf("Set failed: %d", w.Code)
	}

	// Toggle without accentColor.
	updatePayload := map[string]interface{}{
		"public":  true,
		"enabled": false,
		"title":   "Color Preserve",
	}
	w = httptest.NewRecorder()
	spH.Toggle(w, makeRequest("PATCH", "/api/status-pages/color-preserve", "color-preserve", updatePayload))
	if w.Code != http.StatusOK {
		t.Fatalf("Update failed: %d", w.Code)
	}

	page, _ := store.GetStatusPageBySlug("color-preserve")
	if page.AccentColor != "#AABBCC" {
		t.Errorf("Expected accentColor to be preserved, got '%s'", page.AccentColor)
	}
}

// ============================================================
// Public response includes favicon
// ============================================================

func TestBranding_FaviconInPublicResponse(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)
	seedPage(t, store, "fav-resp", "Fav Resp", nil, true, true)

	// Set both logo and favicon.
	payload := map[string]interface{}{
		"public":     true,
		"enabled":    true,
		"title":      "Fav Resp",
		"logoUrl":    "https://example.com/logo.png",
		"faviconUrl": "https://example.com/favicon.ico",
	}
	w := httptest.NewRecorder()
	spH.Toggle(w, makeRequest("PATCH", "/api/status-pages/fav-resp", "fav-resp", payload))
	if w.Code != http.StatusOK {
		t.Fatalf("Toggle failed: %d", w.Code)
	}

	// Fetch public status and verify config includes faviconUrl.
	w = httptest.NewRecorder()
	spH.GetPublicStatus(w, makeRequest("GET", "/api/s/fav-resp", "fav-resp", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GetPublicStatus failed: %d", w.Code)
	}

	body := decodeJSON(t, w)
	config, ok := body["config"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected 'config' object in public status response")
	}

	if config["faviconUrl"] != "https://example.com/favicon.ico" {
		t.Errorf("Expected faviconUrl in config response, got '%v'", config["faviconUrl"])
	}
	if config["logoUrl"] != "https://example.com/logo.png" {
		t.Errorf("Expected logoUrl in config response, got '%v'", config["logoUrl"])
	}
}

// ============================================================
// GetAll response includes branding fields
// ============================================================

func TestBranding_GetAll_IncludesBrandingFields(t *testing.T) {
	_, spH := newStatusPageTestEnv(t)

	// Upsert the "all" page with branding config via Toggle.
	payload := map[string]interface{}{
		"public":      true,
		"enabled":     true,
		"title":       "Global Status",
		"logoUrl":     "https://example.com/logo.png",
		"faviconUrl":  "https://example.com/favicon.ico",
		"accentColor": "#112233",
		"theme":       "dark",
	}
	w := httptest.NewRecorder()
	spH.Toggle(w, makeRequest("PATCH", "/api/status-pages/all", "all", payload))
	if w.Code != http.StatusOK {
		t.Fatalf("Toggle failed: %d %s", w.Code, w.Body.String())
	}

	// Fetch all pages.
	w = httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/status-pages", nil)
	spH.GetAll(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetAll failed: %d", w.Code)
	}

	body := decodeJSON(t, w)
	pages, ok := body["pages"].([]interface{})
	if !ok || len(pages) == 0 {
		t.Fatal("Expected pages array in GetAll response")
	}

	// Find the "all" page.
	var allPage map[string]interface{}
	for _, p := range pages {
		pg := p.(map[string]interface{})
		if pg["slug"] == "all" {
			allPage = pg
			break
		}
	}
	if allPage == nil {
		t.Fatal("'all' page not found in GetAll response")
	}

	if allPage["logoUrl"] != "https://example.com/logo.png" {
		t.Errorf("Expected logoUrl in GetAll response, got '%v'", allPage["logoUrl"])
	}
	if allPage["faviconUrl"] != "https://example.com/favicon.ico" {
		t.Errorf("Expected faviconUrl in GetAll response, got '%v'", allPage["faviconUrl"])
	}
	if allPage["accentColor"] != "#112233" {
		t.Errorf("Expected accentColor '#112233' in GetAll response, got '%v'", allPage["accentColor"])
	}
	if allPage["theme"] != "dark" {
		t.Errorf("Expected theme 'dark' in GetAll response, got '%v'", allPage["theme"])
	}
}

// ============================================================
// Full branding round-trip: set → verify → clear → verify
// ============================================================

func TestBranding_FullRoundTrip_SetAndClearLogoAndFavicon(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)
	seedPage(t, store, "roundtrip", "Round Trip", nil, true, true)

	// --- Phase 1: Set both logo and favicon ---
	setPayload := map[string]interface{}{
		"public":     true,
		"enabled":    true,
		"title":      "Round Trip",
		"logoUrl":    "https://example.com/logo.png",
		"faviconUrl": testDataURI,
	}
	w := httptest.NewRecorder()
	spH.Toggle(w, makeRequest("PATCH", "/api/status-pages/roundtrip", "roundtrip", setPayload))
	if w.Code != http.StatusOK {
		t.Fatalf("Phase 1 set failed: %d %s", w.Code, w.Body.String())
	}

	page, _ := store.GetStatusPageBySlug("roundtrip")
	if page.LogoURL != "https://example.com/logo.png" {
		t.Errorf("Phase 1: logoUrl not set, got '%s'", page.LogoURL)
	}
	if page.FaviconURL != testDataURI {
		t.Error("Phase 1: faviconUrl not set")
	}

	// --- Phase 2: Verify they appear in the public API response ---
	w = httptest.NewRecorder()
	spH.GetPublicStatus(w, makeRequest("GET", "/api/s/roundtrip", "roundtrip", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("Phase 2 public fetch failed: %d", w.Code)
	}
	body := decodeJSON(t, w)
	cfg := body["config"].(map[string]interface{})
	if cfg["logoUrl"] != "https://example.com/logo.png" {
		t.Error("Phase 2: logoUrl not in public config")
	}

	// --- Phase 3: Clear both by sending empty strings ---
	clearPayload := map[string]interface{}{
		"public":     true,
		"enabled":    true,
		"title":      "Round Trip",
		"logoUrl":    "",
		"faviconUrl": "",
	}
	w = httptest.NewRecorder()
	spH.Toggle(w, makeRequest("PATCH", "/api/status-pages/roundtrip", "roundtrip", clearPayload))
	if w.Code != http.StatusOK {
		t.Fatalf("Phase 3 clear failed: %d %s", w.Code, w.Body.String())
	}

	page, _ = store.GetStatusPageBySlug("roundtrip")
	if page.LogoURL != "" {
		t.Errorf("Phase 3: expected logoUrl cleared, got '%s'", page.LogoURL)
	}
	if page.FaviconURL != "" {
		t.Errorf("Phase 3: expected faviconUrl cleared, got '%s'", page.FaviconURL)
	}
}
