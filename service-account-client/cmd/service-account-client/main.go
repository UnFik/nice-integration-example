// Command service-account-client is an example machine caller of the NICE service nadi: it proves the API key of a
// Service Account to NICE and calls the service with the token NICE issued.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/nice-platform/example/service-account-client/internal/bootstrap"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := bootstrap.Run(ctx, os.Args[1:], os.LookupEnv, ".env", os.Stdout, os.Stderr, log)
	stop()
	os.Exit(code)
}
