// Package bootstrap assembles the agent: configuration, token verification, the exchange client, the call use
// case, and the HTTP server. It is the only place that knows every layer.
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/ABA-Developer/nice-auth-sdk/credentials"
	"github.com/ABA-Developer/nice-auth-sdk/verify"

	mcpadapter "github.com/nice-platform/example/agent-backend/internal/adapter/mcp"
	"github.com/nice-platform/example/agent-backend/internal/adapter/niceexchange"
	callusecase "github.com/nice-platform/example/agent-backend/internal/app/call/usecase"
	"github.com/nice-platform/example/agent-backend/internal/domain"
	"github.com/nice-platform/example/agent-backend/internal/platform/config"
)

// Run loads the configuration and serves until ctx ends.
func Run(ctx context.Context, lookup config.LookupFunc, dotenv string, log *slog.Logger) error {
	cfg, err := config.Load(lookup, dotenv)
	if err != nil {
		return err
	}
	// niceauth/verify writes the JWKS copy through a temporary file in the same directory.
	if err := os.MkdirAll(filepath.Dir(cfg.JWKSFile), 0o700); err != nil {
		return fmt.Errorf("JWKS directory: %w", err)
	}
	v, err := verify.New(VerifierConfig(cfg, log))
	if err != nil {
		return err
	}
	go v.Run(ctx)
	// One client for NICE and the services: the exchange and the tool call share its connection pool.
	calls, err := NewCallService(cfg, &http.Client{Timeout: 30 * time.Second}, log)
	if err != nil {
		return err
	}

	srv := &http.Server{Addr: cfg.Listen, Handler: NewHandler(v, calls, cfg), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	log.Info("agen listening", slog.Any("config", cfg), slog.String("mcp", cfg.PublicURL+mcpadapter.MCPPath))
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// VerifierConfig wires niceauth/verify for the agent. Tests reuse it and only swap the key source.
func VerifierConfig(cfg config.Config, log *slog.Logger) verify.Config {
	return verify.Config{
		Issuer: cfg.Issuer, Service: domain.ServiceSlug, JWKSFile: cfg.JWKSFile, Log: log,
		// A caller with more permissions than a token carries gets perms_ref in place of perms. The resolver asks NICE
		// once for the list it names with the token of the request itself, so it needs no credential of NICE.
		Resolver: verify.NewHTTPResolver(cfg.Issuer),
	}
}

// NewCallService wires the call use case to the exchange client of the agent's own service client: the service map,
// the operations, and the calls all go through it. No service address is configured: each comes from the catalog.
func NewCallService(cfg config.Config, hc *http.Client, log *slog.Logger) (*callusecase.Service, error) {
	x, err := niceexchange.New(credentials.ExchangeConfig{
		Issuer: cfg.Issuer, ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret, HTTP: hc, Log: log,
	})
	if err != nil {
		return nil, err
	}
	return &callusecase.Service{
		Discoverer: x,
		Operator:   x,
		ClientVars: config.EnvClientID + " and " + config.EnvClientSecret,
		Log:        log,
	}, nil
}

// NewHandler assembles the HTTP routes of the agent.
func NewHandler(v *verify.Verifier, calls *callusecase.Service, cfg config.Config) http.Handler {
	return mcpadapter.NewHandler(mcpadapter.Deps{Verifier: v, Calls: calls, Issuer: cfg.Issuer, PublicURL: cfg.PublicURL})
}
