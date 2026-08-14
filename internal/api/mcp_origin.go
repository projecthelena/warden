package api

import (
	"net/http"
	"net/url"
	"strings"
)

// requireSameOrigin rejects browser-driven cross-origin calls to the MCP endpoint.
//
// MCP clients are not browsers and send no Origin header, so this costs them nothing. A
// request that does carry one is a web page, and since this endpoint also accepts the
// dashboard's session cookie, a page on another origin driving it would be acting as the
// signed-in user. The MCP spec requires the check for the same reason (DNS rebinding).
func requireSameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || !strings.EqualFold(u.Host, r.Host) {
				http.Error(w, "cross-origin requests are not allowed", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
