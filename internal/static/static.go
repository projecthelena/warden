package static

import (
	"embed"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
)

//go:embed dist/*
var dist embed.FS

// Handler serves the compiled frontend assets. It falls back to index.html for SPA routes.
func Handler() http.Handler {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "frontend assets not found - build the web app", http.StatusNotFound)
		})
	}

	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upath := strings.TrimPrefix(r.URL.Path, "/")
		if upath == "" {
			upath = "index.html"
		}

		// If file exists, serve it
		if _, err := fs.Stat(sub, upath); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}

		// Fallback to index.html for SPA
		// Deep copy the URL to avoid modifying the original request unexpectedly (though here it's fine)
		// and explicitely set path to index.html
		r2 := new(http.Request)
		*r2 = *r
		r2.URL = new(url.URL)
		*r2.URL = *r.URL
		r2.URL.Path = "/"

		// Important: Clear RawQuery if specific behavior needed, or keep it.
		// fileServer uses URL.Path.

		fileServer.ServeHTTP(w, r2)
	})
}
