package api

import (
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// requestLogger logs requests like chi's logger, except for the Kubernetes probe endpoints.
// Liveness and readiness probes hit /healthz and /readyz every few seconds, so logging a
// successful one just buries the real requests. A failing probe is worth seeing (readyz
// returns 503 when the database is unreachable), so those are still logged.
func requestLogger(next http.Handler) http.Handler {
	logged := middleware.Logger(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" && r.URL.Path != "/readyz" {
			logged.ServeHTTP(w, r)
			return
		}
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()
		next.ServeHTTP(ww, r)
		if ww.Status() >= http.StatusBadRequest {
			log.Printf("%s %s -> %d in %s", sanitizeLog(r.Method), sanitizeLog(r.URL.Path), ww.Status(), time.Since(start)) // #nosec G706 -- sanitized
		}
	})
}
