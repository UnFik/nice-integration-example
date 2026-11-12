// Command agent-backend is an example MCP server for the NICE service agen: every tool calls another service on
// behalf of the user of the MCP request, with a token from token exchange (RFC 8693).
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
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

func run(_ context.Context, lookup lookupFunc, dotenv string, _ *slog.Logger) error {
	_, err := loadConfig(lookup, dotenv)
	return err
}
