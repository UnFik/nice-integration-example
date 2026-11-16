// Package system serves the public endpoints that are not item operations.
package system

import "net/http"

// Register mounts /healthz and /openapi.json. Both need no token.
func Register(mux *http.ServeMux, openAPIDoc []byte) {
	mux.HandleFunc("GET /healthz", Healthz)
	mux.HandleFunc("GET /openapi.json", OpenAPI(openAPIDoc))
}
