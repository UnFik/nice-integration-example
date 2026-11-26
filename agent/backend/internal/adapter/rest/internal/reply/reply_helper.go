// Package reply writes the JSON answers of the REST handlers.
package reply

import (
	"encoding/json"
	"net/http"
)

// errorBody has the shape niceauth/verify uses for its own refusals, so a client
// reads every error of the agent the same way.
type errorBody struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func Error(w http.ResponseWriter, status int, code, description string) {
	JSON(w, status, errorBody{Error: code, ErrorDescription: description})
}
