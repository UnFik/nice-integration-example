// Command shared-service-oauth is an example NICE SharedService in nice_token
// mode: clients call it directly with a NICE access token, and it verifies the
// token itself with pkg/nicetoken.
//
//	@title						Inventaris
//	@version					1.0
//	@description				Example SharedService of NICE (auth mode nice_token): stock items per organization. Every item operation needs a NICE access token for the audience inventaris.
//	@BasePath					/
//	@tag.name					items
//	@tag.description			Stock items of the organization of the token
//	@x-nice-roles				[{"id":"operator","name":"Operator","description":"Read, create and edit items","permissions":["inventaris.items.read","inventaris.items.create","inventaris.items.edit"]}]
//
// swag reads every line after @securityDefinitions as part of it, so it stays last.
//
//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				"Bearer <JWT>" from NICE, requested with the scope nice:aud:inventaris.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/nice-platform/nice/pkg/nicetoken"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.LookupEnv, ".env", log); err != nil {
		log.Error("inventaris stopped", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run(ctx context.Context, lookup lookupFunc, dotenv string, log *slog.Logger) error {
	cfg, err := loadConfig(lookup, dotenv)
	if err != nil {
		return err
	}
	// nicetoken writes the JWKS copy through a temporary file in the same directory.
	if err := os.MkdirAll(filepath.Dir(cfg.JWKSFile), 0o700); err != nil {
		return fmt.Errorf("JWKS directory: %w", err)
	}
	store := NewStore()
	v, err := nicetoken.New(verifierConfig(cfg, store, log))
	if err != nil {
		return err
	}
	go v.Run(ctx)
	if err := startFeed(ctx, v, cfg, log); err != nil {
		return err
	}

	srv := &http.Server{Addr: cfg.Listen, Handler: newHandler(v, store, cfg.CORSOrigins), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	log.Info("inventaris listening", slog.String("addr", cfg.Listen), slog.String("issuer", cfg.Issuer),
		slog.Any("cors_origins", cfg.CORSOrigins), slog.Bool("feed", cfg.ServiceAPIKey != ""))
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// verifierConfig wires nicetoken for this service. Tests reuse it and only swap
// the key source and the checker.
func verifierConfig(cfg Config, store *Store, log *slog.Logger) nicetoken.Config {
	c := nicetoken.Config{
		Issuer:   cfg.Issuer,
		Service:  serviceSlug,
		JWKSFile: cfg.JWKSFile,
		// A valid token for this service already proves that NICE entitled the
		// organization, so its first request is a safe moment to create the tenant.
		Tenants: nicetoken.ProvisionOnFirstUse(store.CreateTenant),
		Log:     log,
	}
	if cfg.ServiceAPIKey != "" {
		// Without a checker, nicetoken refuses sensitive operations with 503 (fail closed).
		c.Checker = nicetoken.NewHTTPChecker(cfg.Issuer, cfg.ServiceAPIKey)
	}
	return c
}

// startFeed reads the access change feed when the nice.service key is set, so
// tokens issued before a revocation are refused with token_stale.
func startFeed(ctx context.Context, v *nicetoken.Verifier, cfg Config, log *slog.Logger) error {
	if cfg.ServiceAPIKey == "" {
		log.Warn(envServiceAPIKey + " is not set: the access change feed is off, so revoked access is refused only when the token expires (up to 30 minutes), and DELETE /items/{id} answers 503")
		return nil
	}
	return v.StartFeed(ctx, nicetoken.FeedConfig{APIKey: cfg.ServiceAPIKey})
}
