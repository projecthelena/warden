package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/projecthelena/warden/internal/config"
	"github.com/projecthelena/warden/internal/db"
	"github.com/projecthelena/warden/internal/uptime"
	"github.com/go-chi/chi/v5"
)

// newStatusPageTestEnv creates a fresh test environment for status page tests.
func newStatusPageTestEnv(t *testing.T) (*db.Store, *StatusPageHandler) {
	t.Helper()
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("Failed to create test store: %v", err)
	}
	manager := uptime.NewManager(store)
	authH := NewAuthHandler(store, &config.Config{}, nil)
	spH := NewStatusPageHandler(store, manager, authH)
	return store, spH
}

// seedGroup creates a group in the test store, failing the test on error.
func seedGroup(t *testing.T, store *db.Store, id, name string) {
	t.Helper()
	if err := store.CreateGroup(db.Group{ID: id, Name: name}); err != nil {
		t.Fatalf("Failed to create group %s: %v", id, err)
	}
}

// seedMonitor creates a monitor in the test store, failing the test on error.
func seedMonitor(t *testing.T, store *db.Store, id, groupID, name string) {
	t.Helper()
	if err := store.CreateMonitor(db.Monitor{ID: id, GroupID: groupID, Name: name, Active: true}); err != nil {
		t.Fatalf("Failed to create monitor %s: %v", id, err)
	}
}

// seedPage upserts a status page in the test store, failing the test on error.
func seedPage(t *testing.T, store *db.Store, slug, title string, groupID *string, public, enabled bool) {
	t.Helper()
	if err := store.UpsertStatusPage(slug, title, groupID, public, enabled); err != nil {
		t.Fatalf("Failed to upsert status page %s: %v", slug, err)
	}
}

// seedAuthUser creates a user and session, returning a cookie value for auth.
func seedAuthUser(t *testing.T, store *db.Store, username, token string) {
	t.Helper()
	if err := store.CreateUser(username, "password123", "UTC"); err != nil {
		t.Fatalf("Failed to create user %s: %v", username, err)
	}
	user, err := store.Authenticate(username, "password123")
	if err != nil {
		t.Fatalf("Failed to authenticate user %s: %v", username, err)
	}
	if err := store.CreateSession(user.ID, token, time.Now().Add(24*time.Hour)); err != nil {
		t.Fatalf("Failed to create session for %s: %v", username, err)
	}
}

// makeRequest creates a request with chi URL params for status page handlers.
func makeRequest(method, path, slug string, body interface{}) *http.Request {
	var reqBody *bytes.Buffer
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(b)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, path, reqBody)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("slug", slug)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	return req
}

// decodeJSON decodes a JSON response body into a map.
func decodeJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to decode response JSON: %v (body: %s)", err, w.Body.String())
	}
	return result
}

// --- GetPublicStatus Tests ---

func TestGetPublicStatus_EnabledPublic(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	seedGroup(t, store, "g1", "G1")
	seedMonitor(t, store, "m1", "g1", "M1")
	seedPage(t, store, "all", "Global Status", nil, true, true)

	w := httptest.NewRecorder()
	spH.GetPublicStatus(w, makeRequest("GET", "/api/s/all", "all", nil))

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	body := decodeJSON(t, w)
	if body["public"] != true {
		t.Error("Expected public=true in response")
	}
	if body["title"] != "Global Status" {
		t.Errorf("Expected title='Global Status', got '%v'", body["title"])
	}
}

func TestGetPublicStatus_EnabledPrivate_Unauthenticated(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	seedPage(t, store, "private-page", "Private Page", nil, false, true)

	w := httptest.NewRecorder()
	spH.GetPublicStatus(w, makeRequest("GET", "/api/s/private-page", "private-page", nil))

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", w.Code)
	}

	body := decodeJSON(t, w)
	if body["error"] != "authentication required" {
		t.Errorf("Expected error='authentication required', got '%v'", body["error"])
	}
}

func TestGetPublicStatus_EnabledPrivate_Authenticated(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	seedPage(t, store, "private-auth", "Private Auth", nil, false, true)
	seedAuthUser(t, store, "testadmin", "test-token-123")

	req := makeRequest("GET", "/api/s/private-auth", "private-auth", nil)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: "test-token-123"})

	w := httptest.NewRecorder()
	spH.GetPublicStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 for authenticated user on private page, got %d (body: %s)", w.Code, w.Body.String())
	}

	body := decodeJSON(t, w)
	if body["public"] != false {
		t.Error("Expected public=false in response for private page")
	}
}

func TestGetPublicStatus_DisabledPublic(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	// enabled=false, public=true — should still 404
	seedPage(t, store, "disabled-pub", "Disabled Public", nil, true, false)

	w := httptest.NewRecorder()
	spH.GetPublicStatus(w, makeRequest("GET", "/api/s/disabled-pub", "disabled-pub", nil))

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for disabled page, got %d", w.Code)
	}
}

func TestGetPublicStatus_DisabledPrivate(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	// enabled=false, public=false — should 404 (not 401)
	seedPage(t, store, "disabled-priv", "Disabled Private", nil, false, false)

	w := httptest.NewRecorder()
	spH.GetPublicStatus(w, makeRequest("GET", "/api/s/disabled-priv", "disabled-priv", nil))

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for disabled page (not 401), got %d", w.Code)
	}
}

func TestGetPublicStatus_DisabledEvenForAuthenticated(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	seedPage(t, store, "disabled-auth", "Disabled Auth", nil, true, false)
	seedAuthUser(t, store, "admin2", "auth-token-456")

	req := makeRequest("GET", "/api/s/disabled-auth", "disabled-auth", nil)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: "auth-token-456"})

	w := httptest.NewRecorder()
	spH.GetPublicStatus(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for disabled page even with auth, got %d", w.Code)
	}
}

func TestGetPublicStatus_NonexistentSlug(t *testing.T) {
	_, spH := newStatusPageTestEnv(t)

	w := httptest.NewRecorder()
	spH.GetPublicStatus(w, makeRequest("GET", "/api/s/does-not-exist", "does-not-exist", nil))

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for nonexistent slug, got %d", w.Code)
	}
}

func TestGetPublicStatus_ResponseIncludesGroups(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	seedGroup(t, store, "g-resp", "Response Group")
	seedMonitor(t, store, "m-resp", "g-resp", "Resp Monitor")
	seedPage(t, store, "resp-test", "Response Test", nil, true, true)

	w := httptest.NewRecorder()
	spH.GetPublicStatus(w, makeRequest("GET", "/api/s/resp-test", "resp-test", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	body := decodeJSON(t, w)
	groups, ok := body["groups"].([]interface{})
	if !ok {
		t.Fatal("Expected 'groups' array in response")
	}
	if len(groups) == 0 {
		t.Error("Expected at least one group in response")
	}

	incidents, ok := body["incidents"].([]interface{})
	if !ok {
		t.Fatal("Expected 'incidents' array in response")
	}
	_ = incidents // Just verify it's present and is an array
}

func TestGetPublicStatus_GroupSpecificPage(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	seedGroup(t, store, "g-target", "Target Group")
	seedGroup(t, store, "g-other", "Other Group")
	seedMonitor(t, store, "m-target", "g-target", "Target Monitor")
	seedMonitor(t, store, "m-other", "g-other", "Other Monitor")

	gid := "g-target"
	seedPage(t, store, "target-only", "Target Only", &gid, true, true)

	w := httptest.NewRecorder()
	spH.GetPublicStatus(w, makeRequest("GET", "/api/s/target-only", "target-only", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	body := decodeJSON(t, w)
	groups := body["groups"].([]interface{})
	if len(groups) != 1 {
		t.Errorf("Expected exactly 1 group for group-specific page, got %d", len(groups))
	}
	if len(groups) > 0 {
		g := groups[0].(map[string]interface{})
		if g["name"] != "Target Group" {
			t.Errorf("Expected group 'Target Group', got '%v'", g["name"])
		}
	}
}

// --- Toggle Handler Tests ---

func TestToggle_SetEnabledAndPublic(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	seedPage(t, store, "toggle-test", "Toggle Test", nil, false, false)

	payload := map[string]interface{}{
		"public":  true,
		"enabled": true,
		"title":   "Toggle Test",
	}
	w := httptest.NewRecorder()
	spH.Toggle(w, makeRequest("PATCH", "/api/status-pages/toggle-test", "toggle-test", payload))

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	p, _ := store.GetStatusPageBySlug("toggle-test")
	if !p.Public {
		t.Error("Expected public=true")
	}
	if !p.Enabled {
		t.Error("Expected enabled=true")
	}
}

func TestToggle_EnableOnly(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	seedPage(t, store, "enable-only", "Enable Only", nil, false, false)

	payload := map[string]interface{}{
		"public":  false,
		"enabled": true,
		"title":   "Enable Only",
	}
	w := httptest.NewRecorder()
	spH.Toggle(w, makeRequest("PATCH", "/api/status-pages/enable-only", "enable-only", payload))

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	p, _ := store.GetStatusPageBySlug("enable-only")
	if p.Public {
		t.Error("Expected public=false (unchanged)")
	}
	if !p.Enabled {
		t.Error("Expected enabled=true")
	}
}

func TestToggle_DisableKeepsPublicState(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	seedPage(t, store, "disable-test", "Disable Test", nil, true, true)

	payload := map[string]interface{}{
		"public":  true,
		"enabled": false,
		"title":   "Disable Test",
	}
	w := httptest.NewRecorder()
	spH.Toggle(w, makeRequest("PATCH", "/api/status-pages/disable-test", "disable-test", payload))

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	p, _ := store.GetStatusPageBySlug("disable-test")
	if !p.Public {
		t.Error("Expected public=true (preserved)")
	}
	if p.Enabled {
		t.Error("Expected enabled=false")
	}
}

func TestToggle_InvalidBody(t *testing.T) {
	_, spH := newStatusPageTestEnv(t)

	req := httptest.NewRequest("PATCH", "/api/status-pages/test", bytes.NewBufferString("not json"))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("slug", "test")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	spH.Toggle(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
}

func TestToggle_UpsertCreatesNewPage(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	// Toggle a page that doesn't exist yet — should be created via upsert
	payload := map[string]interface{}{
		"public":  true,
		"enabled": true,
		"title":   "Brand New",
	}
	w := httptest.NewRecorder()
	spH.Toggle(w, makeRequest("PATCH", "/api/status-pages/brand-new", "brand-new", payload))

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	p, _ := store.GetStatusPageBySlug("brand-new")
	if p == nil {
		t.Fatal("Expected page to be created via upsert")
	}
	if p.Title != "Brand New" {
		t.Errorf("Expected title 'Brand New', got '%s'", p.Title)
	}
	if !p.Enabled || !p.Public {
		t.Error("Expected enabled=true, public=true")
	}
}

// --- GetAll Handler Tests ---

func TestGetAll_IncludesEnabledField(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	seedPage(t, store, "all", "Global Status", nil, true, true)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/status-pages", nil)
	spH.GetAll(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	body := decodeJSON(t, w)
	pages := body["pages"].([]interface{})
	if len(pages) == 0 {
		t.Fatal("Expected at least one page")
	}

	// Find the 'all' page
	for _, p := range pages {
		page := p.(map[string]interface{})
		if page["slug"] == "all" {
			if page["enabled"] != true {
				t.Error("Expected enabled=true for 'all' page")
			}
			if page["public"] != true {
				t.Error("Expected public=true for 'all' page")
			}
			return
		}
	}
	t.Error("'all' page not found in GetAll response")
}

func TestGetAll_UnconfiguredGroupDefaultsToDisabled(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	// Create a group but don't configure a status page for it
	seedGroup(t, store, "g-uncfg", "Unconfigured Group")

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/status-pages", nil)
	spH.GetAll(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	body := decodeJSON(t, w)
	pages := body["pages"].([]interface{})

	for _, p := range pages {
		page := p.(map[string]interface{})
		if page["groupId"] == "g-uncfg" {
			if page["enabled"] != false {
				t.Error("Unconfigured group should default to enabled=false")
			}
			if page["public"] != false {
				t.Error("Unconfigured group should default to public=false")
			}
			return
		}
	}
	t.Error("Unconfigured group not found in GetAll response")
}

func TestGetAll_ConfiguredGroupReflectsState(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	seedGroup(t, store, "g-cfg", "Configured Group")
	gid := "g-cfg"
	seedPage(t, store, "cfg-slug", "Configured Page", &gid, true, true)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/status-pages", nil)
	spH.GetAll(w, req)

	body := decodeJSON(t, w)
	pages := body["pages"].([]interface{})

	for _, p := range pages {
		page := p.(map[string]interface{})
		if page["slug"] == "cfg-slug" {
			if page["enabled"] != true {
				t.Error("Expected enabled=true for configured page")
			}
			if page["public"] != true {
				t.Error("Expected public=true for configured page")
			}
			if page["title"] != "Configured Page" {
				t.Errorf("Expected title 'Configured Page', got '%v'", page["title"])
			}
			return
		}
	}
	t.Error("Configured page not found in GetAll response")
}

// --- Integration: Toggle then GetPublicStatus ---

func TestIntegration_EnableThenAccess(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	// Start disabled
	seedPage(t, store, "integ-page", "Integration", nil, true, false)

	// Verify 404
	w := httptest.NewRecorder()
	spH.GetPublicStatus(w, makeRequest("GET", "/api/s/integ-page", "integ-page", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404 before enabling, got %d", w.Code)
	}

	// Enable via Toggle
	payload := map[string]interface{}{
		"public":  true,
		"enabled": true,
		"title":   "Integration",
	}
	w = httptest.NewRecorder()
	spH.Toggle(w, makeRequest("PATCH", "/api/status-pages/integ-page", "integ-page", payload))
	if w.Code != http.StatusOK {
		t.Fatalf("Toggle failed: %d", w.Code)
	}

	// Now it should be accessible
	w = httptest.NewRecorder()
	spH.GetPublicStatus(w, makeRequest("GET", "/api/s/integ-page", "integ-page", nil))
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 after enabling, got %d", w.Code)
	}
}

func TestIntegration_DisableThenBlock(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	// Start enabled + public
	seedPage(t, store, "integ-disable", "Disable Me", nil, true, true)

	// Verify accessible
	w := httptest.NewRecorder()
	spH.GetPublicStatus(w, makeRequest("GET", "/api/s/integ-disable", "integ-disable", nil))
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	// Disable via Toggle
	payload := map[string]interface{}{
		"public":  true,
		"enabled": false,
		"title":   "Disable Me",
	}
	w = httptest.NewRecorder()
	spH.Toggle(w, makeRequest("PATCH", "/api/status-pages/integ-disable", "integ-disable", payload))
	if w.Code != http.StatusOK {
		t.Fatalf("Toggle failed: %d", w.Code)
	}

	// Now it should 404
	w = httptest.NewRecorder()
	spH.GetPublicStatus(w, makeRequest("GET", "/api/s/integ-disable", "integ-disable", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404 after disabling, got %d", w.Code)
	}
}

func TestIntegration_MakePrivateThenRequireAuth(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	// Start enabled + public
	seedPage(t, store, "integ-priv", "Make Private", nil, true, true)

	// Make private via Toggle
	payload := map[string]interface{}{
		"public":  false,
		"enabled": true,
		"title":   "Make Private",
	}
	w := httptest.NewRecorder()
	spH.Toggle(w, makeRequest("PATCH", "/api/status-pages/integ-priv", "integ-priv", payload))
	if w.Code != http.StatusOK {
		t.Fatalf("Toggle failed: %d", w.Code)
	}

	// Unauthenticated -> 401
	w = httptest.NewRecorder()
	spH.GetPublicStatus(w, makeRequest("GET", "/api/s/integ-priv", "integ-priv", nil))
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", w.Code)
	}

	// Authenticated -> 200
	seedAuthUser(t, store, "privuser", "priv-token")

	req := makeRequest("GET", "/api/s/integ-priv", "integ-priv", nil)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: "priv-token"})

	w = httptest.NewRecorder()
	spH.GetPublicStatus(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 for authenticated user, got %d (body: %s)", w.Code, w.Body.String())
	}
}
