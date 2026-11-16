// Package reply writes JSON responses and reads the tenant for the REST handlers.
package reply

import (
	"encoding/json"
	"net/http"

	restdto "github.com/nice-platform/example/shared-service-oauth/internal/adapter/rest/dto"
	"github.com/nice-platform/nice/pkg/nicetoken"
)

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func Error(w http.ResponseWriter, status int, code, description string) {
	JSON(w, status, restdto.ErrorBody{Error: code, ErrorDescription: description})
}

// TenantOf returns the tenant nicetoken resolved from nice_org_id. The tenant
// never comes from a header, parameter or body of the client.
func TenantOf(w http.ResponseWriter, r *http.Request) (string, bool) {
	org, ok := nicetoken.TenantFromContext(r.Context())
	if !ok || org == "" {
		// Fail closed: without a tenant every organization would share one set of items.
		Error(w, http.StatusServiceUnavailable, "temporarily_unavailable", "tenant_unavailable")
		return "", false
	}
	return org, true
}
