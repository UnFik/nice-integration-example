package web

import (
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
)

// NewHandler serves the files of dir. A path that is not a file and has no
// extension is a route of the UI and gets index.html; /auth/ and /devices stay
// 404 so a mistyped API path is not answered with a page.
func NewHandler(dir string) http.Handler {
	root := os.DirFS(dir)
	files := http.FileServerFS(root)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := path.Clean(r.URL.Path)
		if strings.HasPrefix(p, "/auth/") || p == "/devices" || strings.HasPrefix(p, "/devices/") {
			http.NotFound(w, r)
			return
		}
		name := strings.TrimPrefix(p, "/")
		if name != "" {
			if st, err := fs.Stat(root, name); err == nil && !st.IsDir() {
				files.ServeHTTP(w, r)
				return
			} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
				http.Error(w, "the UI could not be read", http.StatusInternalServerError)
				return
			}
			if path.Ext(name) != "" {
				http.NotFound(w, r)
				return
			}
		}
		// index.html is never cached, so a new build shows at once.
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFileFS(w, r, root, "index.html")
	})
}
