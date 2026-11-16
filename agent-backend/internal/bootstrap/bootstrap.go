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

	"github.com/nice-platform/nice/pkg/niceclient"
	"github.com/nice-platform/nice/pkg/nicetoken"

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
	// nicetoken writes the JWKS copy through a temporary file in the same directory.
	if err := os.MkdirAll(filepath.Dir(cfg.JWKSFile), 0o700); err != nil {
		return fmt.Errorf("JWKS directory: %w", err)
	}
	v, err := nicetoken.New(VerifierConfig(cfg, log))
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

// VerifierConfig wires nicetoken for the agent. Tests reuse it and only swap the key source.
func VerifierConfig(cfg config.Config, log *slog.Logger) nicetoken.Config {
	return nicetoken.Config{Issuer: cfg.Issuer, Service: domain.ServiceSlug, JWKSFile: cfg.JWKSFile, Log: log}
}

// NewCallService wires the call use case to the exchange client of the agent's own service client.
func NewCallService(cfg config.Config, hc *http.Client, log *slog.Logger) (*callusecase.Service, error) {
	sender, err := niceexchange.New(niceclient.ExchangeConfig{
		Issuer: cfg.Issuer, ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret, HTTP: hc, Log: log,
	})
	if err != nil {
		return nil, err
	}
	return &callusecase.Service{
		Sender: sender,
		Bases: map[domain.Target]string{
			domain.Inventaris: cfg.InventarisURL,
			domain.Pengiriman: cfg.ProxyURL + "/" + string(domain.Pengiriman),
		},
		ClientVars: config.EnvClientID + " and " + config.EnvClientSecret,
		Log:        log,
	}, nil
}

// NewHandler assembles the HTTP routes of the agent.
func NewHandler(v *nicetoken.Verifier, calls *callusecase.Service, cfg config.Config) http.Handler {
	return mcpadapter.NewHandler(mcpadapter.Deps{Verifier: v, Calls: calls, Issuer: cfg.Issuer, PublicURL: cfg.PublicURL})
}
