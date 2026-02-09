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
	err = s.UpsertStatusPage("custom-slug", "Custom Page", nil, true, true)
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
	if !p.Enabled {
		t.Error("Expected enabled=true")
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

func TestStatusPages_DefaultSeedDisabled(t *testing.T) {
	s := newTestStore(t)

	// The default seeded 'all' page should have enabled=false (DEFAULT FALSE in migration)
	p, err := s.GetStatusPageBySlug("all")
	if err != nil {
		t.Fatalf("GetStatusPageBySlug failed: %v", err)
	}
	if p == nil {
		t.Fatal("Default page missing")
	}
	if p.Enabled {
		t.Error("Default seeded page should have enabled=false")
	}
	if p.Public {
		t.Error("Default seeded page should have public=false")
	}
}

func TestStatusPages_UpsertAllCombinations(t *testing.T) {
	s := newTestStore(t)

	tests := []struct {
		slug    string
		public  bool
		enabled bool
	}{
		{"combo-ff", false, false},
		{"combo-ft", false, true},
		{"combo-tf", true, false},
		{"combo-tt", true, true},
	}

	for _, tc := range tests {
		if err := s.UpsertStatusPage(tc.slug, "Page "+tc.slug, nil, tc.public, tc.enabled); err != nil {
			t.Fatalf("UpsertStatusPage(%s) failed: %v", tc.slug, err)
		}

		p, err := s.GetStatusPageBySlug(tc.slug)
		if err != nil {
			t.Fatalf("GetStatusPageBySlug(%s) failed: %v", tc.slug, err)
		}
		if p == nil {
			t.Fatalf("Page %s not found", tc.slug)
		}
		if p.Public != tc.public {
			t.Errorf("Page %s: expected public=%v, got %v", tc.slug, tc.public, p.Public)
		}
		if p.Enabled != tc.enabled {
			t.Errorf("Page %s: expected enabled=%v, got %v", tc.slug, tc.enabled, p.Enabled)
		}
	}
}

func TestStatusPages_UpsertUpdatesExisting(t *testing.T) {
	s := newTestStore(t)

	// Create page
	if err := s.UpsertStatusPage("update-test", "Original", nil, false, false); err != nil {
		t.Fatal(err)
	}

	// Update it
	if err := s.UpsertStatusPage("update-test", "Updated", nil, true, true); err != nil {
		t.Fatal(err)
	}

	p, _ := s.GetStatusPageBySlug("update-test")
	if p.Title != "Updated" {
		t.Errorf("Expected title 'Updated', got '%s'", p.Title)
	}
	if !p.Public {
		t.Error("Expected public=true after update")
	}
	if !p.Enabled {
		t.Error("Expected enabled=true after update")
	}
}

func TestStatusPages_ToggleEnabledIndependently(t *testing.T) {
	s := newTestStore(t)

	if err := s.UpsertStatusPage("toggle-en", "Toggle Enabled", nil, true, false); err != nil {
		t.Fatal(err)
	}

	// Toggle enabled on
	if err := s.ToggleStatusPageEnabled("toggle-en", true); err != nil {
		t.Fatal(err)
	}

	p, _ := s.GetStatusPageBySlug("toggle-en")
	if !p.Enabled {
		t.Error("Expected enabled=true after toggle")
	}
	if !p.Public {
		t.Error("Public should remain true (unchanged)")
	}

	// Toggle enabled off
	if err := s.ToggleStatusPageEnabled("toggle-en", false); err != nil {
		t.Fatal(err)
	}

	p, _ = s.GetStatusPageBySlug("toggle-en")
	if p.Enabled {
		t.Error("Expected enabled=false after toggle off")
	}
	if !p.Public {
		t.Error("Public should still be true (unchanged)")
	}
}

func TestStatusPages_TogglePublicIndependently(t *testing.T) {
	s := newTestStore(t)

	if err := s.UpsertStatusPage("toggle-pub", "Toggle Public", nil, false, true); err != nil {
		t.Fatal(err)
	}

	// Toggle public on
	if err := s.ToggleStatusPage("toggle-pub", true); err != nil {
		t.Fatal(err)
	}

	p, _ := s.GetStatusPageBySlug("toggle-pub")
	if !p.Public {
		t.Error("Expected public=true after toggle")
	}
	if !p.Enabled {
		t.Error("Enabled should remain true (unchanged)")
	}

	// Toggle public off
	if err := s.ToggleStatusPage("toggle-pub", false); err != nil {
		t.Fatal(err)
	}

	p, _ = s.GetStatusPageBySlug("toggle-pub")
	if p.Public {
		t.Error("Expected public=false after toggle off")
	}
	if !p.Enabled {
		t.Error("Enabled should still be true (unchanged)")
	}
}

func TestStatusPages_GetStatusPagesReturnsEnabled(t *testing.T) {
	s := newTestStore(t)

	if err := s.UpsertStatusPage("list-test", "List Test", nil, true, true); err != nil {
		t.Fatal(err)
	}

	pages, err := s.GetStatusPages()
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, p := range pages {
		if p.Slug == "list-test" {
			found = true
			if !p.Enabled {
				t.Error("Expected enabled=true in GetStatusPages result")
			}
			if !p.Public {
				t.Error("Expected public=true in GetStatusPages result")
			}
			break
		}
	}
	if !found {
		t.Error("Page 'list-test' not found in GetStatusPages")
	}
}

func TestStatusPages_GetBySlugNonexistent(t *testing.T) {
	s := newTestStore(t)

	p, err := s.GetStatusPageBySlug("nonexistent-slug-xyz")
	if err != nil {
		t.Fatalf("Expected nil error for nonexistent slug, got: %v", err)
	}
	if p != nil {
		t.Error("Expected nil page for nonexistent slug")
	}
}

func TestStatusPages_WithGroupID(t *testing.T) {
	s := newTestStore(t)

	if err := s.CreateGroup(Group{ID: "g-sp-test", Name: "SP Group"}); err != nil {
		t.Fatal(err)
	}

	gid := "g-sp-test"
	if err := s.UpsertStatusPage("group-page", "Group Page", &gid, true, true); err != nil {
		t.Fatal(err)
	}

	p, _ := s.GetStatusPageBySlug("group-page")
	if p == nil {
		t.Fatal("Page not found")
	}
	if p.GroupID == nil || *p.GroupID != "g-sp-test" {
		t.Error("Expected groupId='g-sp-test'")
	}
	if !p.Enabled {
		t.Error("Expected enabled=true")
	}
}
