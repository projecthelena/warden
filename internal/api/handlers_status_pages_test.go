package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/clusteruptime/clusteruptime/internal/db"
	"github.com/clusteruptime/clusteruptime/internal/uptime"
	"github.com/go-chi/chi/v5"
)

func TestPublicStatusPage(t *testing.T) {
	// Custom setup since we need Manager for status
	store, _ := db.NewStore(":memory:")
	manager := uptime.NewManager(store)
	spH := NewStatusPageHandler(store, manager)

	// Seed Data
	if err := store.CreateGroup(db.Group{ID: "g1", Name: "G1"}); err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}
	if err := store.CreateMonitor(db.Monitor{ID: "m1", GroupID: "g1", Name: "M1", Active: true}); err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}

	// Default page exists via migration/seed, but we are in :memory: without calling main or router.
	// Store NewStore calls migrate which seeds default page "all".
	// Ensure it is public
	if err := store.UpsertStatusPage("all", "Global Status", nil, true); err != nil {
		t.Fatalf("Failed to upsert status page: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/s/all", nil)

	// Inject Chi URL Param
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("slug", "all")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	spH.GetPublicStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
	// Check body implies "operational"
}

func TestToggleStatusPage(t *testing.T) {
	store, _ := db.NewStore(":memory:")
	manager := uptime.NewManager(store)
	spH := NewStatusPageHandler(store, manager)

	// Create a page
	if err := store.UpsertStatusPage("mypage", "My Page", nil, true); err != nil {
		t.Fatalf("Failed to upsert status page: %v", err)
	}

	// Toggle OFF
	payload := map[string]interface{}{
		"public": false,
		"title":  "My Page",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("PATCH", "/api/status-pages/mypage", bytes.NewBuffer(body))
	// Inject slug
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("slug", "mypage")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	spH.Toggle(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	// Verify DB
	p, _ := store.GetStatusPageBySlug("mypage")
	if p.Public {
		t.Error("Expected public=false")
	}
}
