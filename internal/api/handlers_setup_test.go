package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/clusteruptime/clusteruptime/internal/db"
	"github.com/clusteruptime/clusteruptime/internal/uptime"
	"github.com/go-chi/chi/v5"
)

func TestCheckSetup(t *testing.T) {
	s, _ := db.NewStore(":memory:")
	m := uptime.NewManager(s)

	// Router struct
	r := &Router{
		Mux:     chi.NewRouter(),
		manager: m,
		store:   s,
	}

	req := httptest.NewRequest("GET", "/api/setup/status", nil)
	w := httptest.NewRecorder()

	r.CheckSetup(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
	// Default state is not setup (unless NewStore seeds admin? No, setup does that)
	// Actually NewStore implementation details matter.
	// Assuming initially isSetup=false if no users.
}
