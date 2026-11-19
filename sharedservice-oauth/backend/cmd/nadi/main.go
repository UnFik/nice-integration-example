// Command nadi is the backend of the sharedservice-oauth example: a NICE
// SharedService in nice_token mode that clients call directly with a NICE
// access token, verified with pkg/niceauth/verify, and the backend-for-frontend of
// its UI, which signs users in with pkg/niceauth/credentials and keeps their tokens.
//
//	@title						NADI
//	@version					1.0
//	@description				Example SharedService of NICE (auth mode nice_token): IoT devices per organization and project. Every device operation needs a NICE access token for the audience nadi.
//	@BasePath					/
//	@tag.name					devices
//	@tag.description			IoT devices of the organization and project of the token
//	@x-nice-roles				[{"id":"editor","name":"Editor","description":"Read, create and edit devices","permissions":["nadi.devices.read","nadi.devices.create","nadi.devices.edit"]}]
//
// swag reads every line after @securityDefinitions as part of it, so it stays last.
//
//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				"Bearer <JWT>" from NICE, requested with the scope nice:aud:nadi.
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
		log.Error("nadi stopped", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
