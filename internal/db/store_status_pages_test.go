package db

import (
	"testing"
)

func TestStatusPages(t *testing.T) {
	s := newTestStore(t)

	// global 'all' page seeded by default?
	p, err := s.GetStatusPageBySlug("all")
	if err != nil {
		t.Fatalf("GetStatusPageBySlug 'all' failed: %v", err)
	}
	if p == nil {
		t.Fatal("Default status page 'all' missing")
	}

	// Create Custom
	err = s.UpsertStatusPage("custom-slug", "Custom Page", nil, true)
	if err != nil {
		t.Fatalf("UpsertStatusPage failed: %v", err)
	}

	// Read
	p, err = s.GetStatusPageBySlug("custom-slug")
	if err != nil {
		t.Fatalf("GetStatusPageBySlug failed: %v", err)
	}
	if p == nil {
		t.Fatal("Custom page not found")
	}
	if p.Title != "Custom Page" {
		t.Error("Title mismatch")
	}
	if !p.Public {
		t.Error("Expected public=true")
	}

	// Toggle
	if err := s.ToggleStatusPage("custom-slug", false); err != nil {
		t.Fatalf("ToggleStatusPage failed: %v", err)
	}

	p, _ = s.GetStatusPageBySlug("custom-slug")
	if p.Public {
		t.Error("Expected public=false after toggle")
	}
}
