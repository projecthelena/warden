package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/projecthelena/warden/internal/db"
)

// resetPasswordReq builds a request to POST /api/users/{id}/password routed through a chi
// router carrying the given role, and returns the recorder.
func resetPasswordReq(t *testing.T, store *db.Store, role string, actorID, targetID int64, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	r := chi.NewRouter()
	r.Use(roleAndUserMiddleware(role, actorID))
	r.Post("/api/users/{id}/password", NewUserHandler(store).ResetUserPassword)

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/users/%d/password", targetID), bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestResetUserPassword_AdminResetsUser(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	_ = store.CreateUser("admin1", "OldPass123!", "UTC", "admin")
	_ = store.CreateUser("bob", "OldPass123!", "UTC", "viewer")
	admin, _ := store.Authenticate("admin1", "OldPass123!")
	bob, _ := store.Authenticate("bob", "OldPass123!")

	// Bob has a live session that the reset must revoke.
	if err := store.CreateSession(bob.ID, "bob-token", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	w := resetPasswordReq(t, store, RoleAdmin, admin.ID, bob.ID, map[string]string{"password": "BrandNew123!"})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Old password no longer works, new one does.
	if _, err := store.Authenticate("bob", "OldPass123!"); err == nil {
		t.Error("old password should no longer authenticate")
	}
	if _, err := store.Authenticate("bob", "BrandNew123!"); err != nil {
		t.Errorf("new password should authenticate: %v", err)
	}

	// Bob's session was revoked.
	if sess, _ := store.GetSession("bob-token"); sess != nil {
		t.Error("reset should have revoked existing sessions")
	}
}

func TestResetUserPassword_Rejections(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	_ = store.CreateUser("admin1", "OldPass123!", "UTC", "admin")
	_ = store.CreateUser("bob", "OldPass123!", "UTC", "viewer")
	admin, _ := store.Authenticate("admin1", "OldPass123!")
	bob, _ := store.Authenticate("bob", "OldPass123!")

	// Weak password -> 400, and Bob's password is untouched.
	w := resetPasswordReq(t, store, RoleAdmin, admin.ID, bob.ID, map[string]string{"password": "weak"})
	if w.Code != http.StatusBadRequest {
		t.Errorf("weak password: expected 400, got %d", w.Code)
	}
	if _, err := store.Authenticate("bob", "OldPass123!"); err != nil {
		t.Error("a rejected reset must leave the old password working")
	}

	// Unknown user -> 404.
	w = resetPasswordReq(t, store, RoleAdmin, admin.ID, 99999, map[string]string{"password": "BrandNew123!"})
	if w.Code != http.StatusNotFound {
		t.Errorf("missing user: expected 404, got %d", w.Code)
	}

	// Editor is not allowed near this endpoint -> 403.
	w = resetPasswordReq(t, store, RoleEditor, admin.ID, bob.ID, map[string]string{"password": "BrandNew123!"})
	if w.Code != http.StatusForbidden {
		t.Errorf("editor: expected 403, got %d", w.Code)
	}
}

func TestResetUserPassword_RoleGate(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	_ = store.CreateUser("bob", "OldPass123!", "UTC", "viewer")
	bob, _ := store.Authenticate("bob", "OldPass123!")

	// Every role below admin, and a request that carries no role at all, is refused.
	for _, role := range []string{RoleEditor, RoleViewer, RoleStatusViewer, ""} {
		w := resetPasswordReq(t, store, role, 1, bob.ID, map[string]string{"password": "BrandNew123!"})
		if w.Code != http.StatusForbidden {
			t.Errorf("role %q: expected 403, got %d", role, w.Code)
		}
	}
	// The password never changed through any of those attempts.
	if _, err := store.Authenticate("bob", "OldPass123!"); err != nil {
		t.Error("a refused reset must leave the password intact")
	}
}

func TestResetUserPassword_OnlyRevokesTarget(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	_ = store.CreateUser("admin1", "OldPass123!", "UTC", "admin")
	_ = store.CreateUser("bob", "OldPass123!", "UTC", "viewer")
	_ = store.CreateUser("carol", "OldPass123!", "UTC", "viewer")
	admin, _ := store.Authenticate("admin1", "OldPass123!")
	bob, _ := store.Authenticate("bob", "OldPass123!")
	carol, _ := store.Authenticate("carol", "OldPass123!")
	_ = store.CreateSession(bob.ID, "bob-token", time.Now().Add(time.Hour))
	_ = store.CreateSession(carol.ID, "carol-token", time.Now().Add(time.Hour))

	w := resetPasswordReq(t, store, RoleAdmin, admin.ID, bob.ID, map[string]string{"password": "BrandNew123!"})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if sess, _ := store.GetSession("bob-token"); sess != nil {
		t.Error("bob's session should be revoked")
	}
	if sess, _ := store.GetSession("carol-token"); sess == nil {
		t.Error("carol's session must survive a reset of bob")
	}
}
