// Package niceexchange wraps niceclient: the token exchange of RFC 8693 and the call to a service with the
// exchanged token.
package niceexchange

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/nice-platform/nice/pkg/niceclient"

	calldto "github.com/nice-platform/example/agent-backend/internal/app/call/dto"
	"github.com/nice-platform/example/agent-backend/internal/domain"
)

const maxBody = 1 << 20

// Client sends requests with an exchanged token. It fills the Sender contract of the call use case.
type Client struct {
	x *niceclient.ExchangeClient
}

// New makes the client of the confidential service client bound to agen, the actor of every exchange. hc is shared
// by the exchange and the calls, so they share its connection pool.
func New(cfg niceclient.ExchangeConfig) (*Client, error) {
	x, err := niceclient.NewExchange(cfg)
	if err != nil {
		return nil, err
	}
	return &Client{x: x}, nil
}

// Send is the only place that touches OAuth: DoAs keeps the token per (user, organization, project, service),
// shares one exchange among parallel calls of one key, and sends the request once more after a fresh exchange when
// the service answers 401 token_stale.
func (c *Client) Send(ctx context.Context, token string, key calldto.Key, r calldto.Request) (calldto.Response, error) {
	var rd io.Reader
	if r.Body != nil {
		// A bytes.Reader gives the request a GetBody, so DoAs can send it again after token_stale.
		rd = bytes.NewReader(r.Body)
	}
	req, err := http.NewRequestWithContext(ctx, r.Method, r.URL, rd)
	if err != nil {
		return calldto.Response{}, fmt.Errorf("%w: %v", domain.ErrBuildRequest, err)
	}
	req.Header.Set("Accept", "application/json")
	if r.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.x.DoAs(ctx, token, niceclient.Key{Org: key.Org, Project: key.Project, Audience: key.Audience}, req)
	if err != nil {
		return calldto.Response{}, domainError(err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return calldto.Response{}, domain.ErrIncompleteAnswer
	}
	return calldto.Response{Status: resp.StatusCode, Body: raw}, nil
}

// domainError names a failure of the exchange in the terms of the use case.
func domainError(err error) error {
	var grant *niceclient.GrantError
	var urlErr *url.Error
	switch {
	case errors.As(err, &grant):
		return &domain.GrantError{Code: grant.Code, Description: grant.Description}
	case niceclient.IsTemporary(err):
		return domain.ErrTemporary
	case errors.As(err, &urlErr):
		return domain.ErrUnreachable
	default:
		return err
	}
}
