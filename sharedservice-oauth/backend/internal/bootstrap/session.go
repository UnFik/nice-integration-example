package bootstrap

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/ABA-Developer/nice-auth-sdk/credentials"
	"github.com/ABA-Developer/nice-auth-sdk/verify"
	sessionmemory "github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/memory/session"
	nicegateway "github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/nice"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/auth"
	sessionusecase "github.com/nice-platform/example/sharedservice-oauth/backend/internal/app/session/usecase"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/platform/config"
)

// Auth is the browser side of the service: niceauth/credentials for the confidential
// client, the session use cases and the /auth handler.
type Auth struct {
	Client   *credentials.Client
	Sessions *sessionusecase.Service
	Store    *sessionmemory.Store
	Handler  *auth.Handler
}

// NewAuth wires the sign-in of the UI. Refresh tokens are kept in a
// credentials.MemoryStore and sessions in memory, so a restart signs everybody
// out; the README says what a production store needs.
func NewAuth(cfg config.Config, v *verify.Verifier, log *slog.Logger) (*Auth, error) {
	hc := &http.Client{Timeout: 15 * time.Second}
	c, err := credentials.New(credentials.Config{
		Issuer: cfg.Issuer, ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret,
		Kind: credentials.KindBFF, BaseScopes: nicegateway.BaseScopes, HTTP: hc, Log: log,
	})
	if err != nil {
		return nil, err
	}
	x, err := credentials.NewExchange(credentials.ExchangeConfig{
		Issuer: cfg.Issuer, ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret, HTTP: hc, Log: log,
	})
	if err != nil {
		return nil, err
	}
	store := sessionmemory.NewStore()
	sessions := &sessionusecase.Service{
		Store: store,
		NICE:  &nicegateway.Gateway{Client: c, Exchange: x, Issuer: cfg.Issuer, RedirectURI: cfg.RedirectURI, HTTP: hc},
		Idle:  cfg.SessionIdle,
		Stale: func(ctx context.Context, token string) bool { return stale(ctx, v, token) },
	}
	h := &auth.Handler{
		Sessions:  sessions,
		PublicURL: cfg.PublicURL,
		Log:       log,
		Perms: func(ctx context.Context, token string) ([]string, error) {
			claims, err := v.Verify(ctx, token)
			if err != nil {
				return nil, err
			}
			return claims.Permissions, nil
		},
	}
	return &Auth{Client: c, Sessions: sessions, Store: store, Handler: h}, nil
}

// stale reports whether v refuses token only because the access change feed
// marked its user, organization or project after it was issued. Other refusals
// are not cured by a new token, so they are left to the verifier of the request.
func stale(ctx context.Context, v *verify.Verifier, token string) bool {
	_, err := v.Verify(ctx, token)
	var e *verify.Error
	return errors.As(err, &e) && e.Description == verify.ReasonStale
}

// Start renews the tokens in use and signs out idle sessions until ctx ends.
func (a *Auth) Start(ctx context.Context) {
	go a.Client.Run(ctx)
	go func() {
		t := time.NewTicker(time.Minute)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				a.Sessions.Sweep(ctx)
			}
		}
	}()
}
