// Package domain holds the rules of agen that no framework changes: its identity in the NICE catalog, the services
// its tools call, and what an organization or project ID must look like.
package domain

import "regexp"

// ServiceSlug is the slug registered in the NICE catalog. Tokens for the agent carry it as aud.
const ServiceSlug = "agen"

// PermUse is the permission every call to the MCP endpoint needs.
const PermUse = ServiceSlug + ".mcp.use"

// Target is a service the tools call. Its slug is the audience of the exchanged token.
type Target string

const Inventaris Target = "inventaris"

var (
	orgID     = regexp.MustCompile(`^org_[A-Za-z0-9-]{1,64}$`)
	projectID = regexp.MustCompile(`^prj_[A-Za-z0-9-]{1,64}$`)
)

// ValidOrgID reports whether s looks like an organization ID. The IDs come from the model and become scopes, so a
// space must never pass: it would add a scope of the model's choosing.
func ValidOrgID(s string) bool { return orgID.MatchString(s) }

// ValidProjectID reports whether s looks like a project ID.
func ValidProjectID(s string) bool { return projectID.MatchString(s) }
