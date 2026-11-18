// Package system serves the public endpoints that are not device operations.
package system

import "net/http"

// Register mounts /healthz, /openapi.json and /docs. None needs a token.
func Register(mux *http.ServeMux, openAPIDoc []byte) {
	mux.HandleFunc("GET /healthz", Healthz)
	mux.HandleFunc("GET /openapi.json", OpenAPI(openAPIDoc))
	mux.HandleFunc("GET /docs", Docs)
}
