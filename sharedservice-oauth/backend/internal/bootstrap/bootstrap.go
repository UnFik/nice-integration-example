// Package bootstrap wires the inventory service: configuration, nicetoken
// verifier, storage, use cases and the HTTP server.
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

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest"
	itemusecase "github.com/nice-platform/example/sharedservice-oauth/backend/internal/app/item/usecase"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/platform/config"
	"github.com/nice-platform/nice/pkg/nicetoken"
)

// Run loads the configuration, assembles the service and serves until ctx ends.
func Run(ctx context.Context, lookup config.LookupFunc, dotenv string, log *slog.Logger) error {
	cfg, err := config.Load(lookup, dotenv)
	if err != nil {
		return err
	}
	// nicetoken writes the JWKS copy through a temporary file in the same directory.
	if err := os.MkdirAll(filepath.Dir(cfg.JWKSFile), 0o700); err != nil {
		return fmt.Errorf("JWKS directory: %w", err)
	}
	repo, closeStore, err := OpenItemStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer closeStore()
	v, err := nicetoken.New(VerifierConfig(cfg, repo, log))
	if err != nil {
		return err
	}
	go v.Run(ctx)
	if err := StartFeed(ctx, v, cfg, log); err != nil {
		return err
	}

	handler := rest.NewHandler(v, &itemusecase.Service{Repo: repo}, cfg.CORSOrigins)
	srv := &http.Server{Addr: cfg.Listen, Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	log.Info("inventaris listening", slog.String("addr", cfg.Listen), slog.String("issuer", cfg.Issuer),
		slog.Any("cors_origins", cfg.CORSOrigins), slog.Bool("feed", cfg.ServiceAPIKey != ""), slog.String("store", StoreName(cfg)))
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
