package system

import (
	"encoding/json"
	"net/http"
)

// Healthz never calls NICE, so the NICE health probe sees this service alone.
func Healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
