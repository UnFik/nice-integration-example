// Command inventaris is the backend of the sharedservice-oauth example: a NICE
// SharedService in nice_token mode that clients call directly with a NICE
// access token, verified with pkg/nicetoken, and the backend-for-frontend of
// its UI, which signs users in with pkg/niceclient and keeps their tokens.
//
//	@title						Inventaris
//	@version					1.0
//	@description				Example SharedService of NICE (auth mode nice_token): stock items per organization and project. Every item operation needs a NICE access token for the audience inventaris.
//	@BasePath					/
//	@tag.name					items
//	@tag.description			Stock items of the organization and project of the token
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
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/bootstrap"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := bootstrap.Run(ctx, os.LookupEnv, ".env", log); err != nil {
		log.Error("inventaris stopped", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
