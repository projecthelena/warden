package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/clusteruptime/clusteruptime/internal/db"
	"github.com/clusteruptime/clusteruptime/internal/uptime"
)

func TestGetSystemEvents(t *testing.T) {
	s, _ := db.NewStore(":memory:")
	m := uptime.NewManager(s)
	h := NewEventHandler(s, m)

	req := httptest.NewRequest("GET", "/api/events", nil)
	w := httptest.NewRecorder()

	h.GetSystemEvents(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}
