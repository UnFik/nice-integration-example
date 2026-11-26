// Package niceexchange wraps niceauth/credentials in exchange mode: the service map and the operations of a service
// as NICE lists them for the user, and one call of an operation with a token exchanged on behalf of the user.
package niceexchange

import (
	"context"
	"errors"
	"io"
	"net/url"
	"strings"

	"github.com/ABA-Developer/nice-auth-sdk/credentials"

	calldto "github.com/nice-platform/example/agent/backend/internal/app/call/dto"
	"github.com/nice-platform/example/agent/backend/internal/domain"
)

const maxBody = 1 << 20

// Client is the exchange client of the confidential service client bound to agen, the actor of every exchange. It
// fills the Discoverer and Operator contracts of the call use case.
type Client struct {
	x *credentials.ExchangeClient
}

// New makes the client. The HTTP client of cfg is shared by NICE and the services, so they share its connection pool.
func New(cfg credentials.ExchangeConfig) (*Client, error) {
	x, err := credentials.NewExchange(cfg)
	if err != nil {
		return nil, err
	}
	return &Client{x: x}, nil
}

// CallOperation is the only place a service is called. The SDK builds the address from the catalog, keeps the token
// per (user, organization, project, service), shares one exchange among parallel calls of one key, and sends the call
// once more after a fresh exchange when the service answers 401 token_stale.
func (c *Client) CallOperation(ctx context.Context, token string, call calldto.OperationCall) (calldto.OperationResult, error) {
	op := credentials.OperationCall{
		Service: credentials.MapService{Slug: call.Service.Slug, Name: call.Service.Name, Description: call.Service.Description,
			Endpoint: call.Service.Endpoint, BasePath: call.Service.BasePath},
		Operation: credentials.ServiceOperation{Key: call.Operation.Key, Method: call.Operation.Method, Path: call.Operation.Path,
			Summary: call.Operation.Summary, Permission: call.Operation.Permission, Tag: call.Operation.Tag, Sensitive: call.Operation.Sensitive},
		Org: call.Org, Project: call.Project, PathParams: call.PathParams, Query: url.Values(call.Query), Body: call.Body,
		Header: map[string][]string{"Accept": {"application/json"}},
	}
	for _, h := range call.Operation.Headers {
		op.Operation.Headers = append(op.Operation.Headers, credentials.OperationHeader{Name: h.Name, Value: h.Value})
	}
	resp, err := c.x.CallOperation(ctx, token, op)
	if err != nil {
		return calldto.OperationResult{}, domainError(err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return calldto.OperationResult{}, domain.ErrIncompleteAnswer
	}
	return calldto.OperationResult{Status: resp.StatusCode, Body: raw}, nil
}

// domainError names a failure of the SDK in the terms of the use case.
func domainError(err error) error {
	var grant *credentials.GrantError
	var api *credentials.APIError
	var urlErr *url.Error
	switch {
	case errors.Is(err, credentials.ErrInvalidOperationCall):
		// The SDK names the parameter, never its value.
		return &domain.InvalidCallError{Reason: strings.TrimPrefix(err.Error(), credentials.ErrInvalidOperationCall.Error()+": ")}
	case errors.As(err, &grant):
		return &domain.GrantError{Code: grant.Code, Description: grant.Description}
	case credentials.IsTemporary(err):
		return domain.ErrTemporary
	case errors.As(err, &api):
		return &domain.APIError{Status: api.Status, Code: api.Code}
	case errors.As(err, &urlErr):
		return domain.ErrUnreachable
	default:
		return err
	}
}
