package main

import (
	"net/http"
	"slices"
)

const (
	corsAllowMethods  = "GET, POST, PUT, DELETE"
	corsAllowHeaders  = "Authorization, Content-Type"
	corsExposeHeaders = "WWW-Authenticate, Retry-After"
)

// withCORS lets the browser origins in origins call the service. It wraps the
// whole handler so a preflight is answered before any token is required; a
// browser never sends Authorization on a preflight.
func withCORS(origins []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Add("Vary", "Origin")
		allowed := slices.Contains(origins, origin)
		preflight := r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != ""
		switch {
		case preflight && allowed:
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Allow-Methods", corsAllowMethods)
			h.Set("Access-Control-Allow-Headers", corsAllowHeaders)
			h.Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
		case preflight:
			w.WriteHeader(http.StatusForbidden)
		default:
			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Expose-Headers", corsExposeHeaders)
			}
			next.ServeHTTP(w, r)
		}
	})
}
