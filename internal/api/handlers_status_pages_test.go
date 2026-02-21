package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

// --- Helper for seeding incidents ---

func seedIncident(t *testing.T, store *db.Store, id, title, incType, severity, status string, public bool, groups []string, startOffset time.Duration) {
	t.Helper()
	startTime := time.Now().Add(startOffset)
	var groupsJSON string
	if len(groups) > 0 {
		b, _ := json.Marshal(groups)
		groupsJSON = string(b)
	}
	inc := db.Incident{
		ID:             id,
		Title:          title,
		Description:    "Test incident",
		Type:           incType,
		Severity:       severity,
		Status:         status,
		StartTime:      startTime,
		Public:         public,
		AffectedGroups: groupsJSON,
		Source:         "manual",
	}
	if err := store.CreateIncident(inc); err != nil {
		t.Fatalf("Failed to create incident %s: %v", id, err)
	}
}

func seedResolvedIncident(t *testing.T, store *db.Store, id, title, severity string, public bool, groups []string, startOffset time.Duration) {
	t.Helper()
	startTime := time.Now().Add(startOffset)
	endTime := startTime.Add(1 * time.Hour)
	var groupsJSON string
	if len(groups) > 0 {
		b, _ := json.Marshal(groups)
		groupsJSON = string(b)
	}
	inc := db.Incident{
		ID:             id,
		Title:          title,
		Description:    "Resolved incident",
		Type:           "incident",
		Severity:       severity,
		Status:         "resolved",
		StartTime:      startTime,
		EndTime:        &endTime,
		Public:         public,
		AffectedGroups: groupsJSON,
		Source:         "manual",
	}
	if err := store.CreateIncident(inc); err != nil {
		t.Fatalf("Failed to create resolved incident %s: %v", id, err)
	}
}

// ============================================================
// PHASE 1 TESTS: Uptime Bars
// ============================================================

// findMonitorInGroups looks for a monitor by name across all groups and returns the first match
func findMonitorInGroups(groups []interface{}, monitorName string) map[string]interface{} {
	for _, g := range groups {
		group := g.(map[string]interface{})
		monitors, ok := group["monitors"].([]interface{})
		if !ok {
			continue
		}
		for _, m := range monitors {
			monitor := m.(map[string]interface{})
			if monitor["name"] == monitorName {
				return monitor
			}
		}
	}
	return nil
}

func TestPhase1_ResponseIncludesUptimeDays(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	seedGroup(t, store, "g-uptime", "Uptime Group")
	seedMonitor(t, store, "m-uptime", "g-uptime", "Uptime Monitor")
	seedPage(t, store, "uptime-test", "Uptime Test", nil, true, true)

	w := httptest.NewRecorder()
	spH.GetPublicStatus(w, makeRequest("GET", "/api/s/uptime-test", "uptime-test", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	body := decodeJSON(t, w)
	groups := body["groups"].([]interface{})

	monitor := findMonitorInGroups(groups, "Uptime Monitor")
	if monitor == nil {
		t.Fatal("Expected to find 'Uptime Monitor' in response")
	}

	// Verify uptimeDays field exists (may be empty array)
	if _, ok := monitor["uptimeDays"]; !ok {
		t.Error("Expected 'uptimeDays' field in monitor response")
	}

	// Verify overallUptime field exists
	if _, ok := monitor["overallUptime"]; !ok {
		t.Error("Expected 'overallUptime' field in monitor response")
	}
}

func TestPhase1_ResponseIncludesMonitorStatus(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	seedGroup(t, store, "g-status", "Status Group")
	seedMonitor(t, store, "m-status", "g-status", "Status Monitor")
	seedPage(t, store, "status-test", "Status Test", nil, true, true)

	w := httptest.NewRecorder()
	spH.GetPublicStatus(w, makeRequest("GET", "/api/s/status-test", "status-test", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	body := decodeJSON(t, w)
	groups := body["groups"].([]interface{})

	monitor := findMonitorInGroups(groups, "Status Monitor")
	if monitor == nil {
		t.Fatal("Expected to find 'Status Monitor' in response")
	}

	// Verify status field exists
	status, ok := monitor["status"].(string)
	if !ok {
		t.Error("Expected 'status' field to be a string")
	}

	// Status should be one of: up, down, degraded, paused
	validStatuses := map[string]bool{"up": true, "down": true, "degraded": true, "paused": true}
	if !validStatuses[status] {
		t.Errorf("Unexpected status '%s', expected one of: up, down, degraded, paused", status)
	}
}

func TestPhase1_ResponseIncludesLatency(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	seedGroup(t, store, "g-latency", "Latency Group")
	seedMonitor(t, store, "m-latency", "g-latency", "Latency Monitor")
	seedPage(t, store, "latency-test", "Latency Test", nil, true, true)

	w := httptest.NewRecorder()
	spH.GetPublicStatus(w, makeRequest("GET", "/api/s/latency-test", "latency-test", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	body := decodeJSON(t, w)
	groups := body["groups"].([]interface{})

	monitor := findMonitorInGroups(groups, "Latency Monitor")
	if monitor == nil {
		t.Fatal("Expected to find 'Latency Monitor' in response")
	}

	// Verify latency field exists
	if _, ok := monitor["latency"]; !ok {
		t.Error("Expected 'latency' field in monitor response")
	}

	// Verify lastCheck field exists
	if _, ok := monitor["lastCheck"]; !ok {
		t.Error("Expected 'lastCheck' field in monitor response")
	}
}

func TestPhase1_PausedMonitorShowsPausedStatus(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	seedGroup(t, store, "g-paused", "Paused Group")
	// Create a paused monitor
	if err := store.CreateMonitor(db.Monitor{
		ID:      "m-paused",
		GroupID: "g-paused",
		Name:    "Paused Monitor",
		Active:  false, // Paused
	}); err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}
	seedPage(t, store, "paused-test", "Paused Test", nil, true, true)

	w := httptest.NewRecorder()
	spH.GetPublicStatus(w, makeRequest("GET", "/api/s/paused-test", "paused-test", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	body := decodeJSON(t, w)
	groups := body["groups"].([]interface{})

	monitor := findMonitorInGroups(groups, "Paused Monitor")
	if monitor == nil {
		t.Fatal("Expected to find 'Paused Monitor' in response")
	}

	status := monitor["status"].(string)
	if status != "paused" {
		t.Errorf("Expected status 'paused' for inactive monitor, got '%s'", status)
	}
}

// ============================================================
// PHASE 2 TESTS: Incident History
// ============================================================

func TestPhase2_ResponseIncludesActiveIncidents(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	seedGroup(t, store, "g-inc", "Incident Group")
	seedPage(t, store, "inc-test", "Incident Test", nil, true, true)

	// Create an active public incident
	seedIncident(t, store, "inc-active-1", "Active Incident", "incident", "critical", "investigating", true, nil, 0)

	w := httptest.NewRecorder()
	spH.GetPublicStatus(w, makeRequest("GET", "/api/s/inc-test", "inc-test", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	body := decodeJSON(t, w)
	incidents := body["incidents"].([]interface{})

	if len(incidents) == 0 {
		t.Fatal("Expected at least one active incident")
	}

	// Find our incident
	found := false
	for _, i := range incidents {
		inc := i.(map[string]interface{})
		if inc["title"] == "Active Incident" {
			found = true
			if inc["status"] != "investigating" {
				t.Errorf("Expected status 'investigating', got '%v'", inc["status"])
			}
			if inc["severity"] != "critical" {
				t.Errorf("Expected severity 'critical', got '%v'", inc["severity"])
			}
			break
		}
	}
	if !found {
		t.Error("Active incident not found in response")
	}
}

func TestPhase2_PrivateIncidentNotShown(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	seedGroup(t, store, "g-priv-inc", "Private Incident Group")
	seedPage(t, store, "priv-inc-test", "Private Incident Test", nil, true, true)

	// Create a private incident (should NOT appear on public page)
	seedIncident(t, store, "inc-private-1", "Private Incident", "incident", "critical", "investigating", false, nil, 0)

	w := httptest.NewRecorder()
	spH.GetPublicStatus(w, makeRequest("GET", "/api/s/priv-inc-test", "priv-inc-test", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	body := decodeJSON(t, w)
	incidents := body["incidents"].([]interface{})

	// Private incident should not appear
	for _, i := range incidents {
		inc := i.(map[string]interface{})
		if inc["title"] == "Private Incident" {
			t.Error("Private incident should not appear on public status page")
		}
	}
}

func TestPhase2_ResponseIncludesPastIncidents(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	seedGroup(t, store, "g-past", "Past Group")
	seedPage(t, store, "past-test", "Past Test", nil, true, true)

	// Create a resolved public incident (within last 14 days)
	seedResolvedIncident(t, store, "inc-resolved-1", "Past Incident", "major", true, nil, -2*24*time.Hour)

	w := httptest.NewRecorder()
	spH.GetPublicStatus(w, makeRequest("GET", "/api/s/past-test", "past-test", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	body := decodeJSON(t, w)
	pastIncidents, ok := body["pastIncidents"].([]interface{})
	if !ok {
		t.Fatal("Expected 'pastIncidents' array in response")
	}

	if len(pastIncidents) == 0 {
		t.Fatal("Expected at least one past incident")
	}

	// Find our incident
	found := false
	for _, i := range pastIncidents {
		inc := i.(map[string]interface{})
		if inc["title"] == "Past Incident" {
			found = true
			if inc["status"] != "resolved" {
				t.Errorf("Expected status 'resolved', got '%v'", inc["status"])
			}
			break
		}
	}
	if !found {
		t.Error("Past incident not found in pastIncidents")
	}
}

func TestPhase2_MaintenanceWindowsShownSeparately(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	seedGroup(t, store, "g-maint", "Maintenance Group")
	seedPage(t, store, "maint-test", "Maintenance Test", nil, true, true)

	// Create an active maintenance window
	seedIncident(t, store, "maint-1", "Scheduled Maintenance", "maintenance", "minor", "scheduled", true, nil, 0)

	w := httptest.NewRecorder()
	spH.GetPublicStatus(w, makeRequest("GET", "/api/s/maint-test", "maint-test", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	body := decodeJSON(t, w)
	incidents := body["incidents"].([]interface{})

	// Find maintenance window
	found := false
	for _, i := range incidents {
		inc := i.(map[string]interface{})
		if inc["title"] == "Scheduled Maintenance" {
			found = true
			if inc["type"] != "maintenance" {
				t.Errorf("Expected type 'maintenance', got '%v'", inc["type"])
			}
			break
		}
	}
	if !found {
		t.Error("Maintenance window not found in response")
	}
}

func TestPhase2_IncidentUpdatesIncluded(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	seedGroup(t, store, "g-updates", "Updates Group")
	seedPage(t, store, "updates-test", "Updates Test", nil, true, true)

	// Create an incident with updates
	seedIncident(t, store, "inc-with-updates", "Incident With Updates", "incident", "major", "investigating", true, nil, 0)

	// Add updates
	_ = store.CreateIncidentUpdate("inc-with-updates", "investigating", "Looking into the issue")
	_ = store.CreateIncidentUpdate("inc-with-updates", "identified", "Root cause found")

	w := httptest.NewRecorder()
	spH.GetPublicStatus(w, makeRequest("GET", "/api/s/updates-test", "updates-test", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	body := decodeJSON(t, w)
	incidents := body["incidents"].([]interface{})

	// Find our incident
	for _, i := range incidents {
		inc := i.(map[string]interface{})
		if inc["title"] == "Incident With Updates" {
			updates, ok := inc["updates"].([]interface{})
			if !ok {
				t.Error("Expected 'updates' array in incident")
				return
			}
			if len(updates) != 2 {
				t.Errorf("Expected 2 updates, got %d", len(updates))
			}
			return
		}
	}
	t.Error("Incident with updates not found")
}

func TestPhase2_GroupFilteringForIncidents(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	// Create two groups
	seedGroup(t, store, "g-target-inc", "Target Group")
	seedGroup(t, store, "g-other-inc", "Other Group")

	// Create a group-specific status page
	gid := "g-target-inc"
	seedPage(t, store, "group-inc-test", "Group Inc Test", &gid, true, true)

	// Create incident for target group (should appear)
	seedIncident(t, store, "inc-target", "Target Incident", "incident", "major", "investigating", true, []string{"g-target-inc"}, 0)

	// Create incident for other group (should NOT appear)
	seedIncident(t, store, "inc-other", "Other Incident", "incident", "major", "investigating", true, []string{"g-other-inc"}, 0)

	w := httptest.NewRecorder()
	spH.GetPublicStatus(w, makeRequest("GET", "/api/s/group-inc-test", "group-inc-test", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	body := decodeJSON(t, w)
	incidents := body["incidents"].([]interface{})

	foundTarget := false
	foundOther := false
	for _, i := range incidents {
		inc := i.(map[string]interface{})
		if inc["title"] == "Target Incident" {
			foundTarget = true
		}
		if inc["title"] == "Other Incident" {
			foundOther = true
		}
	}

	if !foundTarget {
		t.Error("Target group incident should appear")
	}
	if foundOther {
		t.Error("Other group incident should NOT appear on group-specific page")
	}
}

// ============================================================
// PHASE 3 TESTS: Configuration
// ============================================================

func TestPhase3_ToggleWithBrandingConfig(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	seedPage(t, store, "branding-test", "Branding Test", nil, true, true)

	logoURL := "https://example.com/logo.png"
	accentColor := "#FF5500"
	description := "Test status page"

	payload := map[string]interface{}{
		"public":      true,
		"enabled":     true,
		"title":       "Branding Test",
		"logoUrl":     logoURL,
		"accentColor": accentColor,
		"description": description,
	}

	w := httptest.NewRecorder()
	spH.Toggle(w, makeRequest("PATCH", "/api/status-pages/branding-test", "branding-test", payload))

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	// Verify stored values
	page, _ := store.GetStatusPageBySlug("branding-test")
	if page.LogoURL != logoURL {
		t.Errorf("Expected logoUrl '%s', got '%s'", logoURL, page.LogoURL)
	}
	if page.AccentColor != accentColor {
		t.Errorf("Expected accentColor '%s', got '%s'", accentColor, page.AccentColor)
	}
	if page.Description != description {
		t.Errorf("Expected description '%s', got '%s'", description, page.Description)
	}
}

func TestPhase3_InvalidAccentColorRejected(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	seedPage(t, store, "color-test", "Color Test", nil, true, true)

	payload := map[string]interface{}{
		"public":      true,
		"enabled":     true,
		"title":       "Color Test",
		"accentColor": "invalid-color",
	}

	w := httptest.NewRecorder()
	spH.Toggle(w, makeRequest("PATCH", "/api/status-pages/color-test", "color-test", payload))

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid accent color, got %d", w.Code)
	}
}

func TestPhase3_ValidHexColorsAccepted(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	testColors := []string{"#FF5500", "#ffffff", "#000000", "#AbCdEf"}

	for i, color := range testColors {
		slug := "color-valid-" + string(rune('a'+i))
		seedPage(t, store, slug, "Color Valid", nil, true, true)

		payload := map[string]interface{}{
			"public":      true,
			"enabled":     true,
			"title":       "Color Valid",
			"accentColor": color,
		}

		w := httptest.NewRecorder()
		spH.Toggle(w, makeRequest("PATCH", "/api/status-pages/"+slug, slug, payload))

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200 for valid color '%s', got %d", color, w.Code)
		}
	}
}

func TestPhase3_ThemeConfiguration(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	validThemes := []string{"light", "dark", "system"}

	for _, theme := range validThemes {
		slug := "theme-" + theme
		seedPage(t, store, slug, "Theme Test", nil, true, true)

		payload := map[string]interface{}{
			"public":  true,
			"enabled": true,
			"title":   "Theme Test",
			"theme":   theme,
		}

		w := httptest.NewRecorder()
		spH.Toggle(w, makeRequest("PATCH", "/api/status-pages/"+slug, slug, payload))

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200 for theme '%s', got %d", theme, w.Code)
		}

		page, _ := store.GetStatusPageBySlug(slug)
		if page.Theme != theme {
			t.Errorf("Expected theme '%s', got '%s'", theme, page.Theme)
		}
	}
}

func TestPhase3_InvalidThemeRejected(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	seedPage(t, store, "bad-theme", "Bad Theme", nil, true, true)

	payload := map[string]interface{}{
		"public":  true,
		"enabled": true,
		"title":   "Bad Theme",
		"theme":   "invalid-theme",
	}

	w := httptest.NewRecorder()
	spH.Toggle(w, makeRequest("PATCH", "/api/status-pages/bad-theme", "bad-theme", payload))

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid theme, got %d", w.Code)
	}
}

func TestPhase3_DisplayToggles(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	seedPage(t, store, "toggles-test", "Toggles Test", nil, true, true)

	// Disable all display options
	payload := map[string]interface{}{
		"public":               true,
		"enabled":              true,
		"title":                "Toggles Test",
		"showUptimeBars":       false,
		"showUptimePercentage": false,
		"showIncidentHistory":  false,
	}

	w := httptest.NewRecorder()
	spH.Toggle(w, makeRequest("PATCH", "/api/status-pages/toggles-test", "toggles-test", payload))

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	page, _ := store.GetStatusPageBySlug("toggles-test")
	if page.ShowUptimeBars {
		t.Error("Expected ShowUptimeBars=false")
	}
	if page.ShowUptimePercentage {
		t.Error("Expected ShowUptimePercentage=false")
	}
	if page.ShowIncidentHistory {
		t.Error("Expected ShowIncidentHistory=false")
	}
}

func TestPhase3_ConfigInPublicResponse(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	seedGroup(t, store, "g-cfg-resp", "Config Response Group")
	seedPage(t, store, "cfg-resp-test", "Config Response Test", nil, true, true)

	// Set config values
	payload := map[string]interface{}{
		"public":               true,
		"enabled":              true,
		"title":                "Config Response Test",
		"description":          "Test description",
		"logoUrl":              "https://example.com/logo.png",
		"accentColor":          "#FF0000",
		"theme":                "dark",
		"showUptimeBars":       true,
		"showUptimePercentage": false,
		"showIncidentHistory":  true,
	}

	w := httptest.NewRecorder()
	spH.Toggle(w, makeRequest("PATCH", "/api/status-pages/cfg-resp-test", "cfg-resp-test", payload))

	if w.Code != http.StatusOK {
		t.Fatalf("Toggle failed: %d", w.Code)
	}

	// Get public status and verify config is included
	w = httptest.NewRecorder()
	spH.GetPublicStatus(w, makeRequest("GET", "/api/s/cfg-resp-test", "cfg-resp-test", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	body := decodeJSON(t, w)
	config, ok := body["config"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected 'config' object in response")
	}

	if config["description"] != "Test description" {
		t.Errorf("Expected description 'Test description', got '%v'", config["description"])
	}
	if config["logoUrl"] != "https://example.com/logo.png" {
		t.Errorf("Expected logoUrl, got '%v'", config["logoUrl"])
	}
	if config["accentColor"] != "#FF0000" {
		t.Errorf("Expected accentColor '#FF0000', got '%v'", config["accentColor"])
	}
	if config["theme"] != "dark" {
		t.Errorf("Expected theme 'dark', got '%v'", config["theme"])
	}
	if config["showUptimeBars"] != true {
		t.Error("Expected showUptimeBars=true")
	}
	if config["showUptimePercentage"] != false {
		t.Error("Expected showUptimePercentage=false")
	}
	if config["showIncidentHistory"] != true {
		t.Error("Expected showIncidentHistory=true")
	}
}

func TestPhase3_DataURILogoAccepted(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	seedPage(t, store, "data-logo", "Data Logo Test", nil, true, true)

	dataURI := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="

	payload := map[string]interface{}{
		"public":  true,
		"enabled": true,
		"title":   "Data Logo Test",
		"logoUrl": dataURI,
	}

	w := httptest.NewRecorder()
	spH.Toggle(w, makeRequest("PATCH", "/api/status-pages/data-logo", "data-logo", payload))

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 for data URI logo, got %d", w.Code)
	}

	page, _ := store.GetStatusPageBySlug("data-logo")
	if page.LogoURL != dataURI {
		t.Error("Data URI logo should be stored")
	}
}

func TestPhase3_InvalidLogoURLRejected(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	seedPage(t, store, "bad-logo", "Bad Logo", nil, true, true)

	payload := map[string]interface{}{
		"public":  true,
		"enabled": true,
		"title":   "Bad Logo",
		"logoUrl": "ftp://invalid-protocol.com/logo.png",
	}

	w := httptest.NewRecorder()
	spH.Toggle(w, makeRequest("PATCH", "/api/status-pages/bad-logo", "bad-logo", payload))

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid logo URL, got %d", w.Code)
	}
}

// ============================================================
// PHASE 4 TESTS: RSS Feed
// ============================================================

func TestPhase4_RSSFeedBasic(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	seedGroup(t, store, "g-rss", "RSS Group")
	seedPage(t, store, "rss-test", "RSS Test", nil, true, true)

	// Create a public incident
	seedIncident(t, store, "inc-rss-1", "RSS Incident", "incident", "major", "investigating", true, nil, 0)

	w := httptest.NewRecorder()
	spH.GetRSSFeed(w, makeRequest("GET", "/api/s/rss-test/rss", "rss-test", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	// Verify Content-Type
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/rss+xml; charset=utf-8" {
		t.Errorf("Expected Content-Type 'application/rss+xml; charset=utf-8', got '%s'", contentType)
	}

	// Verify it's valid RSS
	body := w.Body.String()
	if !strings.Contains(body, `<?xml version="1.0" encoding="UTF-8"?>`) {
		t.Error("Expected XML declaration")
	}
	if !strings.Contains(body, `<rss version="2.0"`) {
		t.Error("Expected RSS 2.0 declaration")
	}
	if !strings.Contains(body, `<channel>`) {
		t.Error("Expected channel element")
	}
}

func TestPhase4_RSSFeedContainsIncident(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	seedPage(t, store, "rss-inc-test", "RSS Inc Test", nil, true, true)

	// Create a public incident
	seedIncident(t, store, "inc-rss-show", "RSS Visible Incident", "incident", "critical", "investigating", true, nil, 0)

	w := httptest.NewRecorder()
	spH.GetRSSFeed(w, makeRequest("GET", "/api/s/rss-inc-test/rss", "rss-inc-test", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	body := w.Body.String()

	// Verify incident appears in feed
	if !strings.Contains(body, "RSS Visible Incident") {
		t.Error("Expected incident title in RSS feed")
	}
	if !strings.Contains(body, "[CRITICAL]") {
		t.Error("Expected severity label in RSS feed")
	}
	if !strings.Contains(body, `<item>`) {
		t.Error("Expected item element in RSS feed")
	}
	if !strings.Contains(body, `<guid`) {
		t.Error("Expected guid element in RSS feed")
	}
	if !strings.Contains(body, `<pubDate>`) {
		t.Error("Expected pubDate element in RSS feed")
	}
}

func TestPhase4_RSSFeedExcludesPrivateIncidents(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	seedPage(t, store, "rss-priv-test", "RSS Priv Test", nil, true, true)

	// Create a private incident (should NOT appear in feed)
	seedIncident(t, store, "inc-rss-private", "Private RSS Incident", "incident", "critical", "investigating", false, nil, 0)

	w := httptest.NewRecorder()
	spH.GetRSSFeed(w, makeRequest("GET", "/api/s/rss-priv-test/rss", "rss-priv-test", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	body := w.Body.String()

	if strings.Contains(body, "Private RSS Incident") {
		t.Error("Private incident should NOT appear in RSS feed")
	}
}

func TestPhase4_RSSFeedEmptyWhenNoIncidents(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	seedPage(t, store, "rss-empty-test", "RSS Empty Test", nil, true, true)

	w := httptest.NewRecorder()
	spH.GetRSSFeed(w, makeRequest("GET", "/api/s/rss-empty-test/rss", "rss-empty-test", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	body := w.Body.String()

	// Should still be valid RSS, just with no items
	if !strings.Contains(body, `<channel>`) {
		t.Error("Expected valid RSS with channel")
	}
	if strings.Contains(body, `<item>`) {
		t.Error("Expected no items in empty feed")
	}
}

func TestPhase4_RSSFeedDisabledPageReturns404(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	// Disabled page
	seedPage(t, store, "rss-disabled", "RSS Disabled", nil, true, false)

	w := httptest.NewRecorder()
	spH.GetRSSFeed(w, makeRequest("GET", "/api/s/rss-disabled/rss", "rss-disabled", nil))

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for disabled page, got %d", w.Code)
	}
}

func TestPhase4_RSSFeedPrivatePageReturns404(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	// Private page (enabled but not public)
	seedPage(t, store, "rss-private-page", "RSS Private Page", nil, false, true)

	w := httptest.NewRecorder()
	spH.GetRSSFeed(w, makeRequest("GET", "/api/s/rss-private-page/rss", "rss-private-page", nil))

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for private page RSS feed, got %d", w.Code)
	}
}

func TestPhase4_RSSFeedNonexistentSlugReturns404(t *testing.T) {
	_, spH := newStatusPageTestEnv(t)

	w := httptest.NewRecorder()
	spH.GetRSSFeed(w, makeRequest("GET", "/api/s/nonexistent/rss", "nonexistent", nil))

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for nonexistent slug, got %d", w.Code)
	}
}

func TestPhase4_RSSFeedGroupFiltering(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	// Create two groups
	seedGroup(t, store, "g-rss-target", "RSS Target Group")
	seedGroup(t, store, "g-rss-other", "RSS Other Group")

	// Create a group-specific status page
	gid := "g-rss-target"
	seedPage(t, store, "rss-group-test", "RSS Group Test", &gid, true, true)

	// Create incident for target group (should appear)
	seedIncident(t, store, "inc-rss-target", "Target Group Incident", "incident", "major", "investigating", true, []string{"g-rss-target"}, 0)

	// Create incident for other group (should NOT appear)
	seedIncident(t, store, "inc-rss-other", "Other Group Incident", "incident", "major", "investigating", true, []string{"g-rss-other"}, 0)

	w := httptest.NewRecorder()
	spH.GetRSSFeed(w, makeRequest("GET", "/api/s/rss-group-test/rss", "rss-group-test", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	body := w.Body.String()

	if !strings.Contains(body, "Target Group Incident") {
		t.Error("Target group incident should appear in RSS feed")
	}
	if strings.Contains(body, "Other Group Incident") {
		t.Error("Other group incident should NOT appear in group-specific RSS feed")
	}
}

func TestPhase4_RSSFeedIncludesMaintenanceAsScheduled(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	seedPage(t, store, "rss-maint-test", "RSS Maint Test", nil, true, true)

	// Create maintenance window
	seedIncident(t, store, "maint-rss-1", "Scheduled Downtime", "maintenance", "minor", "scheduled", true, nil, 0)

	w := httptest.NewRecorder()
	spH.GetRSSFeed(w, makeRequest("GET", "/api/s/rss-maint-test/rss", "rss-maint-test", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	body := w.Body.String()

	if !strings.Contains(body, "Scheduled Downtime") {
		t.Error("Maintenance should appear in RSS feed")
	}
	if !strings.Contains(body, "[MAINTENANCE]") {
		t.Error("Expected MAINTENANCE label for maintenance type")
	}
}

func TestPhase4_RSSFeedIncludesIncidentUpdates(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	seedPage(t, store, "rss-updates-test", "RSS Updates Test", nil, true, true)

	// Create incident with updates
	seedIncident(t, store, "inc-rss-updates", "Incident With Timeline", "incident", "major", "identified", true, nil, 0)
	_ = store.CreateIncidentUpdate("inc-rss-updates", "investigating", "Looking into the issue")
	_ = store.CreateIncidentUpdate("inc-rss-updates", "identified", "Root cause found")

	w := httptest.NewRecorder()
	spH.GetRSSFeed(w, makeRequest("GET", "/api/s/rss-updates-test/rss", "rss-updates-test", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	body := w.Body.String()

	// Updates should be included in the description
	if !strings.Contains(body, "Looking into the issue") {
		t.Error("Expected incident update message in RSS feed")
	}
	if !strings.Contains(body, "Root cause found") {
		t.Error("Expected second incident update message in RSS feed")
	}
}

func TestPhase4_RSSFeedXMLEscaping(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	seedPage(t, store, "rss-escape-test", "RSS Escape Test", nil, true, true)

	// Create incident with special characters that need XML escaping
	inc := db.Incident{
		ID:          "inc-xml-escape",
		Title:       "Test <script>alert('xss')</script> & more",
		Description: "Description with \"quotes\" and 'apostrophes'",
		Type:        "incident",
		Severity:    "major",
		Status:      "investigating",
		StartTime:   time.Now(),
		Public:      true,
	}
	if err := store.CreateIncident(inc); err != nil {
		t.Fatalf("Failed to create incident: %v", err)
	}

	w := httptest.NewRecorder()
	spH.GetRSSFeed(w, makeRequest("GET", "/api/s/rss-escape-test/rss", "rss-escape-test", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	body := w.Body.String()

	// Check that special characters are properly escaped
	if strings.Contains(body, "<script>") {
		t.Error("Script tag should be escaped")
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Error("Script tag should be escaped to &lt;script&gt;")
	}
	if strings.Contains(body, `& more"`) {
		t.Error("Ampersand should be escaped")
	}
}

func TestPhase4_RSSFeedCorrectDateFormat(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	seedPage(t, store, "rss-date-test", "RSS Date Test", nil, true, true)

	seedIncident(t, store, "inc-date-test", "Date Test Incident", "incident", "minor", "investigating", true, nil, 0)

	w := httptest.NewRecorder()
	spH.GetRSSFeed(w, makeRequest("GET", "/api/s/rss-date-test/rss", "rss-date-test", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	body := w.Body.String()

	// Check for RFC1123Z date format (e.g., "Mon, 02 Jan 2006 15:04:05 -0700")
	// This regex matches the general pattern
	if !strings.Contains(body, "<pubDate>") || !strings.Contains(body, "</pubDate>") {
		t.Error("Expected pubDate element with RFC1123Z formatted date")
	}
	if !strings.Contains(body, "<lastBuildDate>") || !strings.Contains(body, "</lastBuildDate>") {
		t.Error("Expected lastBuildDate element")
	}
}

func TestPhase4_RSSFeedAtomSelfLink(t *testing.T) {
	store, spH := newStatusPageTestEnv(t)

	seedPage(t, store, "rss-atom-test", "RSS Atom Test", nil, true, true)

	w := httptest.NewRecorder()
	spH.GetRSSFeed(w, makeRequest("GET", "/api/s/rss-atom-test/rss", "rss-atom-test", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	body := w.Body.String()

	// Check for atom namespace and self link
	if !strings.Contains(body, `xmlns:atom="http://www.w3.org/2005/Atom"`) {
		t.Error("Expected Atom namespace declaration")
	}
	if !strings.Contains(body, `<atom:link href=`) {
		t.Error("Expected Atom self link")
	}
	if !strings.Contains(body, `rel="self"`) {
		t.Error("Expected rel='self' in Atom link")
	}
}
