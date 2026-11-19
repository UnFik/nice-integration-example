// Package mcpadapter serves the agent over HTTP: the MCP endpoint behind niceauth/verify, the public endpoints, and the
// tools that call other services through the call use case.
package mcpadapter

import (
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"

	"github.com/ABA-Developer/nice-auth-sdk/verify"

	"github.com/nice-platform/example/agent-backend/internal/adapter/mcp/internal/caller"
	"github.com/nice-platform/example/agent-backend/internal/adapter/mcp/internal/challenge"
	callusecase "github.com/nice-platform/example/agent-backend/internal/app/call/usecase"
	"github.com/nice-platform/example/agent-backend/internal/domain"
)

const (
	// MCPPath is the one operation of agen.
	MCPPath      = "/mcp"
	metadataPath = "/.well-known/oauth-protected-resource"
)

// baseScopes are the scopes every NICE login asks for; the MCP client adds nice:prj:<project> itself, because a
// token of a service client needs a project and the agent cannot know which one the user wants.
var baseScopes = []string{"openid", "profile", "email", "offline_access"}

// Deps is what the routes need from the assembly.
type Deps struct {
	Verifier  *verify.Verifier
	Calls     *callusecase.Service
	Issuer    string
	PublicURL string
}

// NewHandler assembles the routes. /healthz, /openapi.json and the resource metadata are public; /mcp verifies the
// NICE token, then agen.mcp.use, then hands the verified token to the MCP server.
func NewHandler(d Deps) http.Handler {
	metadataURL := d.PublicURL + metadataPath
	md := auth.ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
		Resource:               d.PublicURL + MCPPath,
		ResourceName:           "Agen (NICE example MCP server)",
		AuthorizationServers:   []string{d.Issuer},
		ScopesSupported:        append(append([]string{}, baseScopes...), "nice:aud:"+domain.ServiceSlug),
		BearerMethodsSupported: []string{"header"},
	})
	server := newServer(&Handlers{Calls: d.Calls})
	// Stateless with JSON answers: every POST stands alone, which lets curl call a tool in one request and lets
	// tool calls of one client run in parallel.
	mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	// RequireBearerToken is only the way to hand the verified token to the tools: niceauth/verify already decided.
	handOver := auth.RequireBearerToken(caller.TokenInfo, &auth.RequireBearerTokenOptions{ClockSkew: time.Minute})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", Healthz)
	mux.HandleFunc("GET /openapi.json", ServeOpenAPI)
	mux.Handle(metadataPath, md)
	mux.Handle(metadataPath+MCPPath, md)
	mux.Handle(MCPPath, challenge.With(metadataURL, d.Verifier.Middleware(d.Verifier.Require(domain.PermUse)(handOver(mcpHandler)))))
	return mux
}
