package domain

import "errors"

// Scope is the key every item operation runs under. It comes from the verified
// token only: OrgID from nice_org_id, ProjectID from prj, Subject from sub and
// Actor from act.sub. Nothing the client sends may change it.
type Scope struct {
	OrgID     string
	ProjectID string
	Subject   string
	Actor     string
}

var (
	// ErrProjectRequired refuses a token without prj: items belong to a project.
	ErrProjectRequired = errors.New("token has no project")
	// ErrOrgRequired refuses a scope without an organization, so items of
	// different organizations can never share the empty tenant.
	ErrOrgRequired = errors.New("token has no organization")
)

// Validate reports why s cannot hold items.
func (s Scope) Validate() error {
	switch {
	case s.OrgID == "":
		return ErrOrgRequired
	case s.ProjectID == "":
		return ErrProjectRequired
	}
	return nil
}
