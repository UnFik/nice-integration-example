// Package caller carries the user of an MCP request from the HTTP layer to the tools.
package caller

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/ABA-Developer/nice-auth-sdk/verify"

	calldto "github.com/nice-platform/example/agent/backend/internal/app/call/dto"
)

const key = "nice_caller"

var errNoCaller = errors.New("the request carries no verified NICE token")

// TokenInfo hands the token that niceauth/verify verified to the MCP layer, with the caller for the tools.
func TokenInfo(ctx context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
	c, ok := verify.FromContext(ctx)
	if !ok {
		return nil, auth.ErrInvalidToken
	}
	return &auth.TokenInfo{
		UserID:     c.Subject,
		Expiration: time.Unix(c.ExpiresAt, 0),
		Extra:      map[string]any{key: calldto.Caller{Token: token, Subject: c.Subject, OrgID: c.OrgID, ProjectID: c.ProjectID}},
	}, nil
}

// From returns the user of the MCP request req.
func From(req *mcp.CallToolRequest) (calldto.Caller, error) {
	if req == nil || req.Extra == nil || req.Extra.TokenInfo == nil {
		return calldto.Caller{}, errNoCaller
	}
	c, ok := req.Extra.TokenInfo.Extra[key].(calldto.Caller)
	if !ok {
		return calldto.Caller{}, errNoCaller
	}
	return c, nil
}
