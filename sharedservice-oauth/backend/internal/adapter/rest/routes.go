// Package rest assembles the REST API of the inventory service.
package rest

import (
	"net/http"

	"github.com/nice-platform/example/sharedservice-oauth/backend/docs"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/auth"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/item"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/system"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/web"
	itemusecase "github.com/nice-platform/example/sharedservice-oauth/backend/internal/app/item/usecase"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/platform/httpx"
	"github.com/nice-platform/nice/pkg/nicetoken"
)

// ItemRoutes is the table of item operations the tests compare with the OpenAPI document.
func ItemRoutes(items *itemusecase.Service) []item.Route {
	return item.New(items).Routes()
}

// Options are the optional parts of the handler.
type Options struct {
	// Auth serves /auth/* and lets item requests with the session cookie in.
	// nil serves the Bearer API only.
	Auth *auth.Handler
	// WebDir is the built UI served at /; empty or missing serves none.
	WebDir string
	// CORSOrigins are the other browser origins allowed to call with a Bearer token.
	CORSOrigins []string
}

// NewHandler assembles CORS -> mux. /healthz, /openapi.json and /docs are public;
// every item route verifies the token and then its permission. With a session
// cookie, Auth fills the token of the active context before that chain runs.
func NewHandler(v *nicetoken.Verifier, items *itemusecase.Service, o Options) http.Handler {
	mux := http.NewServeMux()
	system.Register(mux, docs.OpenAPI)
	for _, rt := range ItemRoutes(items) {
		var opts []nicetoken.RequireOption
		if rt.Sensitive {
			opts = append(opts, nicetoken.Sensitive())
		}
		h := v.Middleware(v.Require(rt.Permission, opts...)(rt.Handler))
		if o.Auth != nil {
			h = o.Auth.WithSession(h)
		}
		mux.Handle(rt.Method+" "+rt.Pattern, h)
	}
	if o.Auth != nil {
		o.Auth.Register(mux)
	}
	web.Register(mux, o.WebDir)
	return httpx.WithCORS(o.CORSOrigins, mux)
}
