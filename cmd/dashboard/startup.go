package main

import (
	"net/http"
	"sync/atomic"

	"github.com/projecthelena/warden/internal/api"
)

// startupHandler serves while the database opens and migrates: /healthz reports the
// process alive, and every other path, /readyz included, answers 503 until ready hands
// it the router.
type startupHandler struct {
	router atomic.Pointer[http.Handler]
}

func (h *startupHandler) ready(router http.Handler) {
	h.router.Store(&router)
}

func (h *startupHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if router := h.router.Load(); router != nil {
		(*router).ServeHTTP(w, r)
		return
	}
	if r.URL.Path == "/healthz" {
		api.Healthz(w, r)
		return
	}
	w.Header().Set("Retry-After", "5")
	http.Error(w, "Warden is starting: running database migrations", http.StatusServiceUnavailable)
}
