package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/clusteruptime/clusteruptime/internal/db"
)

func TestAPIKeysHandler(t *testing.T) {
	s, _ := db.NewStore(":memory:")
	h := NewAPIKeyHandler(s)

	// List Empty
	req := httptest.NewRequest("GET", "/api/api-keys", nil)
	w := httptest.NewRecorder()
	h.ListKeys(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("ListKeys failed: %d", w.Code)
	}

	// Create
	payload := map[string]string{"name": "TestKey"}
	body, _ := json.Marshal(payload)
	req = httptest.NewRequest("POST", "/api/api-keys", bytes.NewBuffer(body))
	w = httptest.NewRecorder()
	h.CreateKey(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("CreateKey failed: %d", w.Code)
	}

	// Delete (Need ID from list)
	// Mock or verify indirectly via store if needed, or parse body.
}
