// Command agent-backend is an example MCP server for the NICE service agen: every tool calls another service on
// behalf of the user of the MCP request, with a token from token exchange (RFC 8693).
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
		log.Error("agen stopped", slog.String("error", err.Error()))
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
	v, err := nicetoken.New(verifierConfig(cfg, log))
	if err != nil {
		return err
	}
	go v.Run(ctx)
	// One client for NICE and the services: the exchange and the tool call share its connection pool.
	a, err := newAgent(cfg, &http.Client{Timeout: 30 * time.Second}, log)
	if err != nil {
		return err
	}

	srv := &http.Server{Addr: cfg.Listen, Handler: newHandler(v, a, cfg), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	log.Info("agen listening", slog.Any("config", cfg), slog.String("mcp", cfg.PublicURL+mcpPath))
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
