// Package web serves the built UI (web/dist) from the origin of the API, so
// the browser sees one origin and the session cookie needs no CORS.
package web

import (
	"net/http"
	"os"
	"path/filepath"
)

// Register serves dir at / when it holds index.html; without a build the
// agent serves no UI and / is 404 as before.
func Register(mux *http.ServeMux, dir string) bool {
	if dir == "" {
		return false
	}
	if st, err := os.Stat(filepath.Join(dir, "index.html")); err != nil || st.IsDir() {
		return false
	}
	mux.Handle("/", NewHandler(dir))
	return true
}
