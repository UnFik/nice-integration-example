package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"

	"github.com/nice-platform/nice/pkg/nicetoken"
)

const metadataPath = "/.well-known/oauth-protected-resource"

// baseScopes are the scopes every NICE login asks for; the MCP client adds nice:prj:<project> itself, because a
// token of a service client needs a project and the agent cannot know which one the user wants.
var baseScopes = []string{"openid", "profile", "email", "offline_access"}

// verifierConfig wires nicetoken for the agent. Tests reuse it and only swap the key source.
func verifierConfig(cfg Config, log *slog.Logger) nicetoken.Config {
	return nicetoken.Config{Issuer: cfg.Issuer, Service: serviceSlug, JWKSFile: cfg.JWKSFile, Log: log}
}

// newHandler assembles the routes. /healthz, /openapi.json and the resource metadata are public; /mcp verifies the
// NICE token, then agen.mcp.use, then hands the verified token to the MCP server.
func newHandler(v *nicetoken.Verifier, a *agent, cfg Config) http.Handler {
	metadataURL := cfg.PublicURL + metadataPath
	md := auth.ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
		Resource:               cfg.PublicURL + mcpPath,
		ResourceName:           "Agen (NICE example MCP server)",
		AuthorizationServers:   []string{cfg.Issuer},
		ScopesSupported:        append(append([]string{}, baseScopes...), "nice:aud:"+serviceSlug),
		BearerMethodsSupported: []string{"header"},
	})
	// Stateless with JSON answers: every POST stands alone, which lets curl call a tool in one request and lets
	// tool calls of one client run in parallel.
	mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return a.server },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	// RequireBearerToken is only the way to hand the verified token to the tools: nicetoken already decided.
	handOver := auth.RequireBearerToken(tokenInfo, &auth.RequireBearerTokenOptions{ClockSkew: time.Minute})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz)
	mux.HandleFunc("GET /openapi.json", serveOpenAPI)
	mux.Handle(metadataPath, md)
	mux.Handle(metadataPath+mcpPath, md)
	mux.Handle(mcpPath, withChallenge(metadataURL, v.Middleware(v.Require(permUse)(handOver(mcpHandler)))))
	return mux
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}` + "\n"))
}

// caller is the user of one MCP request: the token as it came, for the exchange, and its verified claims.
type caller struct {
	token  string
	claims *nicetoken.Claims
}

const callerKey = "nice_caller"

func tokenInfo(ctx context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
	c, ok := nicetoken.FromContext(ctx)
	if !ok {
		return nil, auth.ErrInvalidToken
	}
	return &auth.TokenInfo{
		UserID:     c.Subject,
		Expiration: time.Unix(c.ExpiresAt, 0),
		Extra:      map[string]any{callerKey: caller{token: token, claims: c}},
	}, nil
}

var errNoCaller = errors.New("the request carries no verified NICE token")

// callerOf returns the user of the MCP request req.
func callerOf(req *mcp.CallToolRequest) (caller, error) {
	if req == nil || req.Extra == nil || req.Extra.TokenInfo == nil {
		return caller{}, errNoCaller
	}
	c, ok := req.Extra.TokenInfo.Extra[callerKey].(caller)
	if !ok || c.claims == nil {
		return caller{}, errNoCaller
	}
	return c, nil
}

// withChallenge adds the resource metadata to the challenges of nicetoken, so an MCP client that gets 401 or 403
// finds the authorization server (RFC 9728, section 5.1).
func withChallenge(metadataURL string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(&challengeWriter{ResponseWriter: w, metadata: `resource_metadata="` + metadataURL + `"`}, r)
	})
}

type challengeWriter struct {
	http.ResponseWriter
	metadata    string
	wroteHeader bool
}

func (c *challengeWriter) WriteHeader(status int) {
	if !c.wroteHeader {
		c.wroteHeader = true
		h := c.Header()
		switch status {
		case http.StatusUnauthorized:
			if v := h.Get("WWW-Authenticate"); v == "" || v == "Bearer" {
				h.Set("WWW-Authenticate", "Bearer "+c.metadata)
			} else {
				h.Set("WWW-Authenticate", v+", "+c.metadata)
			}
		case http.StatusForbidden:
			h.Set("WWW-Authenticate", `Bearer error="insufficient_scope", `+c.metadata)
		}
	}
	c.ResponseWriter.WriteHeader(status)
}

func (c *challengeWriter) Write(b []byte) (int, error) {
	if !c.wroteHeader {
		c.WriteHeader(http.StatusOK)
	}
	return c.ResponseWriter.Write(b)
}

// Flush keeps the streaming of the MCP handler working through the wrapper.
func (c *challengeWriter) Flush() {
	if f, ok := c.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (c *challengeWriter) Unwrap() http.ResponseWriter { return c.ResponseWriter }
