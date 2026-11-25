// Package domain holds the rules of agen that no framework changes: its identity in the NICE catalog, what a service
// slug and an organization or project ID must look like, and where a token may be sent.
package domain

import (
	"net"
	"net/url"
	"regexp"
	"strings"
)

// ServiceSlug is the slug registered in the NICE catalog. Tokens for the agent carry it as aud.
const ServiceSlug = "agen"

// PermUse is the permission every call to the MCP endpoint needs.
const PermUse = ServiceSlug + ".mcp.use"

// Target is the slug of a service a tool calls, as the service map of NICE lists it. It is the audience of the
// exchanged token.
type Target string

var (
	serviceSlug = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)
	orgID       = regexp.MustCompile(`^org_[A-Za-z0-9-]{1,64}$`)
	projectID   = regexp.MustCompile(`^prj_[A-Za-z0-9-]{1,64}$`)
)

// ValidServiceSlug reports whether s looks like a service slug. The slug comes from the model and becomes a scope and
// a path segment of NICE, so a space or a slash must never pass.
func ValidServiceSlug(s string) bool { return serviceSlug.MatchString(s) }

// ValidOrgID reports whether s looks like an organization ID. The IDs come from the model and become scopes, so a
// space must never pass: it would add a scope of the model's choosing.
func ValidOrgID(s string) bool { return orgID.MatchString(s) }

// ValidProjectID reports whether s looks like a project ID.
func ValidProjectID(s string) bool { return projectID.MatchString(s) }

// SecureEndpoint reports whether a token of the user may be sent to endpoint: https, or plain http to a loopback host
// only. The SDK refuses the same; checking first gives the model a clear reason.
func SecureEndpoint(endpoint string) bool {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" {
		return false
	}
	switch u.Scheme {
	case "https":
		return true
	case "http":
		host := u.Hostname()
		if strings.EqualFold(host, "localhost") {
			return true
		}
		ip := net.ParseIP(host)
		return ip != nil && ip.IsLoopback()
	}
	return false
}
