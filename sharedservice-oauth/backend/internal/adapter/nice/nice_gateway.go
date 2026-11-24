// Package nicegateway implements the NICE port of the session use cases with
// nice-auth-sdk/credentials: a Client (KindBFF) that keeps the refresh token of every
// session, and an ExchangeClient of the same confidential client that trades
// the sign-in token for a delegated token of the NICE API.
package nicegateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/ABA-Developer/nice-auth-sdk/credentials"
	sessionusecase "github.com/nice-platform/example/sharedservice-oauth/backend/internal/app/session/usecase"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

// BaseScopes are sent with the sign-in and with every refresh, so userinfo
// keeps the name and email after a switch of organization or project.
var BaseScopes = []string{"openid", "profile", "email", "offline_access"}

type Gateway struct {
	Client      *credentials.Client
	Exchange    *credentials.ExchangeClient
	Issuer      string
	RedirectURI string
	// PostLogoutRedirectURI is where NICE sends the browser after ending its
	// session; it must be registered on the client. Empty shows the page of NICE.
	PostLogoutRedirectURI string
	HTTP                  *http.Client
}

// prompts are the values of prompt the UI may ask the sign-in for; any other
// value signs in without one.
var prompts = map[string]bool{credentials.PromptLogin: true, credentials.PromptSelectAccount: true}

func (g *Gateway) AuthorizeURL(ctx context.Context, subject, session, prompt string) (string, error) {
	if !prompts[prompt] {
		prompt = ""
	}
	return g.Client.AuthorizeURL(ctx, credentials.ConnectRequest{Subject: subject, Session: session, RedirectURI: g.RedirectURI, Prompt: prompt})
}

func (g *Gateway) HandleCallback(ctx context.Context, session string, query url.Values) (string, string, error) {
	conn, err := g.Client.HandleCallback(ctx, session, query)
	var ge *credentials.GrantError
	switch {
	case err == nil:
		return conn.NiceSubject, conn.IDToken, nil
	case errors.Is(err, credentials.ErrStateMismatch):
		return "", "", sessionusecase.ErrStateMismatch
	case errors.As(err, &ge):
		// Every refusal at the callback (access_denied included) leaves no session to end.
		return "", "", &sessionusecase.RefusedError{Code: ge.Code, Description: ge.Description}
	}
	return "", "", mapError(err)
}

func (g *Gateway) EndSessionURL(idTokenHint string) (string, error) {
	return g.Client.EndSessionURL(credentials.EndSessionRequest{PostLogoutRedirectURI: g.PostLogoutRedirectURI, IDTokenHint: idTokenHint})
}

func (g *Gateway) ServiceToken(ctx context.Context, subject string, c domain.Context) (string, error) {
	tok, err := g.Client.Token(ctx, credentials.Key{Subject: subject, Org: c.Org, Audience: domain.ServiceSlug, Project: c.Project})
	if err != nil {
		return "", mapError(err)
	}
	return tok.Access, nil
}

func (g *Gateway) DropServiceToken(_ context.Context, subject string, c domain.Context, token string) {
	g.Client.Invalidate(credentials.Key{Subject: subject, Org: c.Org, Audience: domain.ServiceSlug, Project: c.Project},
		credentials.Token{Access: token})
}

func (g *Gateway) SignInToken(ctx context.Context, subject string) (string, error) {
	tok, err := g.Client.SignInToken(ctx, subject)
	if err != nil {
		return "", mapError(err)
	}
	return tok.Access, nil
}

func (g *Gateway) Disconnect(ctx context.Context, subject string) error {
	return mapError(g.Client.Disconnect(ctx, subject))
}

// mapError turns the errors of niceauth/credentials into those of the use cases.
func mapError(err error) error {
	var ge *credentials.GrantError
	switch {
	case err == nil:
		return nil
	case errors.Is(err, credentials.ErrSessionEnded), errors.Is(err, credentials.ErrNotConnected):
		return sessionusecase.ErrSessionEnded
	case errors.As(err, &ge):
		return &sessionusecase.RefusedError{Code: ge.Code, Description: ge.Description}
	case credentials.IsTemporary(err):
		return &sessionusecase.UnavailableError{Err: err}
	}
	return err
}

// exchangeError maps a refusal of the exchange. It never ends the session:
// invalid_grant there is about the subject token, not the refresh token.
func exchangeError(err error) error {
	var ge *credentials.GrantError
	switch {
	case errors.As(err, &ge):
		return &sessionusecase.RefusedError{Code: ge.Code, Description: ge.Description}
	case credentials.IsTemporary(err):
		return &sessionusecase.UnavailableError{Err: err}
	}
	return &sessionusecase.UnavailableError{Err: err}
}

// answerError reads the OAuth error of an answer of /oauth/userinfo that is not 2xx.
func answerError(resp *http.Response) error {
	var body struct {
		Error       string `json:"error"`
		Description string `json:"error_description"`
	}
	_ = json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&body)
	if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
		return &sessionusecase.UnavailableError{Err: fmt.Errorf("NICE answered %d %s", resp.StatusCode, body.Error)}
	}
	code := body.Error
	if code == "" {
		code = "http_" + strconv.Itoa(resp.StatusCode)
	}
	return &sessionusecase.RefusedError{Code: code, Description: body.Description}
}

var _ sessionusecase.NICE = (*Gateway)(nil)
