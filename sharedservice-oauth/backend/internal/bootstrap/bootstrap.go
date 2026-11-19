// Package bootstrap wires the NADI device service: configuration, niceauth/verify
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
	deviceusecase "github.com/nice-platform/example/sharedservice-oauth/backend/internal/app/device/usecase"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/platform/config"
	"github.com/nice-platform/nice/pkg/niceauth/verify"
)

// Run loads the configuration, assembles the service and serves until ctx ends.
func Run(ctx context.Context, lookup config.LookupFunc, dotenv string, log *slog.Logger) error {
	cfg, err := config.Load(lookup, dotenv)
	if err != nil {
		return err
	}
	// niceauth/verify writes the JWKS copy through a temporary file in the same directory.
	if err := os.MkdirAll(filepath.Dir(cfg.JWKSFile), 0o700); err != nil {
		return fmt.Errorf("JWKS directory: %w", err)
	}
	repo, closeStore, err := OpenDeviceStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer closeStore()
	v, err := verify.New(VerifierConfig(cfg, repo, log))
	if err != nil {
		return err
	}
	go v.Run(ctx)
	if err := StartFeed(ctx, v, cfg, log); err != nil {
		return err
	}

	a, err := NewAuth(cfg, v, log)
	if err != nil {
		return err
	}
	a.Start(ctx)
	handler := rest.NewHandler(v, &deviceusecase.Service{Repo: repo}, rest.Options{Auth: a.Handler, WebDir: cfg.WebDir, CORSOrigins: cfg.CORSOrigins})
	srv := &http.Server{Addr: cfg.Listen, Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	log.Info("nadi listening", slog.String("addr", cfg.Listen), slog.String("issuer", cfg.Issuer),
		slog.String("public_url", cfg.PublicURL), slog.String("client_id", cfg.ClientID), slog.Bool("ui", hasUI(cfg.WebDir)),
		slog.Any("cors_origins", cfg.CORSOrigins), slog.Bool("feed", cfg.ServiceAPIKey != ""), slog.String("store", StoreName(cfg)))
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func hasUI(dir string) bool {
	st, err := os.Stat(filepath.Join(dir, "index.html"))
	return err == nil && !st.IsDir()
}
