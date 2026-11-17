// Package key derives the exchange key of a call.
package key

import (
	calldto "github.com/nice-platform/example/agent-backend/internal/app/call/dto"
	"github.com/nice-platform/example/agent-backend/internal/domain"
)

// For is the exchange key of a call. The IDs come from the model, so they are checked before they become scopes.
func For(c calldto.Caller, t domain.Target, where calldto.Where) (calldto.Key, *calldto.Failure) {
	if where.Org != "" && !domain.ValidOrgID(where.Org) {
		return calldto.Key{}, &calldto.Failure{Code: "invalid_argument", Message: "organization_id must look like org_<id>"}
	}
	if where.Project != "" && !domain.ValidProjectID(where.Project) {
		return calldto.Key{}, &calldto.Failure{Code: "invalid_argument", Message: "project_id must look like prj_<id>"}
	}
	if where.Org == c.OrgID {
		where.Org = ""
	}
	// The agent never picks a project in another organization: the user names it.
	if where.Org != "" && where.Project == "" {
		return calldto.Key{}, &calldto.Failure{Code: "invalid_argument",
			Message: "project_id is required with an organization_id other than the one of the user's token"}
	}
	return calldto.Key{Org: where.Org, Project: where.Project, Audience: string(t)}, nil
}
