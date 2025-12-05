package static

import (
	"embed"
	"io/fs"
	"net/http"
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
		if _, err := fs.Stat(sub, upath); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}
		r2 := *r
		r2.URL.Path = "/index.html"
		fileServer.ServeHTTP(w, &r2)
	})
}
