package mcpadapter

import (
	"net/http"

	"github.com/nice-platform/example/agent-backend/docs"
)

// ServeOpenAPI serves the hand-written catalog document of agen.
func ServeOpenAPI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(docs.OpenAPI)
}
