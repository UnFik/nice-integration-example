package system

import "net/http"

// OpenAPI serves the embedded document NICE imports as the service catalog.
func OpenAPI(doc []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(doc)
	}
}
