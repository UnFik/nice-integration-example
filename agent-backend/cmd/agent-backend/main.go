// Command agent-backend is an example MCP server for the NICE service agen: every tool calls another service on
// behalf of the user of the MCP request, with a token from token exchange (RFC 8693).
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/nice-platform/example/agent-backend/internal/bootstrap"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := bootstrap.Run(ctx, os.LookupEnv, ".env", log); err != nil {
		log.Error("agen stopped", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
