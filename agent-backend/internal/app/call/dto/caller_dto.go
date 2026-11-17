// Package calldto holds the data that crosses the boundary of the call use case.
package calldto

// Caller is the user of one MCP request: the token as it came, for the exchange, and the claims the call needs.
type Caller struct {
	Token   string
	Subject string
	OrgID   string
}

// Where is the organization and project a tool call names; empty keeps those of the user's token.
type Where struct {
	Org     string
	Project string
}

// Key is the exchange key of a call: the token is kept per (user, organization, project, service).
type Key struct {
	Org      string
	Project  string
	Audience string
}
