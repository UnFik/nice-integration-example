// Package rest assembles the REST API of the NADI device service.
package rest

import (
	"net/http"

	"github.com/nice-platform/example/sharedservice-oauth/backend/docs"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/auth"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/device"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/system"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/web"
	deviceusecase "github.com/nice-platform/example/sharedservice-oauth/backend/internal/app/device/usecase"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/platform/httpx"
	"github.com/nice-platform/nice/pkg/nicetoken"
)

// DeviceRoutes is the table of device operations the tests compare with the OpenAPI document.
func DeviceRoutes(devices *deviceusecase.Service) []device.Route {
	return device.New(devices).Routes()
}

// Options are the optional parts of the handler.
type Options struct {
	// Auth serves /auth/* and lets device requests with the session cookie in.
	// nil serves the Bearer API only.
	Auth *auth.Handler
	// WebDir is the built UI served at /; empty or missing serves none.
	WebDir string
	// CORSOrigins are the other browser origins allowed to call with a Bearer token.
	CORSOrigins []string
}

// NewHandler assembles CORS -> mux. /healthz, /openapi.json and /docs are public;
// every device route verifies the token and then its permission. With a session
// cookie, Auth fills the token of the active context before that chain runs.
func NewHandler(v *nicetoken.Verifier, devices *deviceusecase.Service, o Options) http.Handler {
	mux := http.NewServeMux()
	system.Register(mux, docs.OpenAPI)
	for _, rt := range DeviceRoutes(devices) {
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
