// Package reply writes JSON responses and reads the scope for the REST handlers.
package reply

import (
	"encoding/json"
	"net/http"

	restdto "github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/dto"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
	"github.com/nice-platform/nice/pkg/niceauth/verify"
)

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func Error(w http.ResponseWriter, status int, code, description string) {
	JSON(w, status, restdto.ErrorBody{Error: code, ErrorDescription: description})
}

// ScopeOf returns the scope of the verified token: the tenant niceauth/verify
// resolved from nice_org_id, the project from prj, and the user and actor from
// sub and act. The scope never comes from a header, parameter or body of the
// client. A token without prj is refused before the body is read.
func ScopeOf(w http.ResponseWriter, r *http.Request) (domain.Scope, bool) {
	org, ok := verify.TenantFromContext(r.Context())
	claims, hasClaims := verify.FromContext(r.Context())
	if !ok || org == "" || !hasClaims {
		// Fail closed: without a tenant every organization would share one set of devices.
		Error(w, http.StatusServiceUnavailable, "temporarily_unavailable", "tenant_unavailable")
		return domain.Scope{}, false
	}
	s := domain.Scope{OrgID: org, ProjectID: claims.ProjectID, Subject: claims.Subject, Actor: claims.Actor}
	if s.ProjectID == "" {
		Error(w, http.StatusForbidden, "insufficient_scope", ProjectRequired)
		return domain.Scope{}, false
	}
	return s, true
}

// ProjectRequired is the error_description of a token without prj.
const ProjectRequired = "project_required"
