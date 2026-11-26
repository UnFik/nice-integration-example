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
// extension is a route of the UI and gets index.html; /auth/, /chat and /mcp
// stay 404 so a mistyped API path is not answered with a page. Only GET and
// HEAD are served: the pattern is / for every method, next to /mcp.
func NewHandler(dir string) http.Handler {
	root := os.DirFS(dir)
	files := http.FileServerFS(root)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		p := path.Clean(r.URL.Path)
		if strings.HasPrefix(p, "/auth/") || p == "/chat" || strings.HasPrefix(p, "/chat/") || strings.HasPrefix(p, "/mcp") {
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
