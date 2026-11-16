// Package rest assembles the REST API of the inventory service.
package rest

import (
	"net/http"

	"github.com/nice-platform/example/shared-service-oauth/docs"
	"github.com/nice-platform/example/shared-service-oauth/internal/adapter/rest/item"
	"github.com/nice-platform/example/shared-service-oauth/internal/adapter/rest/system"
	itemusecase "github.com/nice-platform/example/shared-service-oauth/internal/app/item/usecase"
	"github.com/nice-platform/example/shared-service-oauth/internal/platform/httpx"
	"github.com/nice-platform/nice/pkg/nicetoken"
)

// ItemRoutes is the table of item operations the tests compare with the OpenAPI document.
func ItemRoutes(items *itemusecase.Service) []item.Route {
	return item.New(items).Routes()
}

// NewHandler assembles CORS -> mux. /healthz, /openapi.json and /docs are public;
// every item route verifies the token and then its permission.
func NewHandler(v *nicetoken.Verifier, items *itemusecase.Service, corsOrigins []string) http.Handler {
	mux := http.NewServeMux()
	system.Register(mux, docs.OpenAPI)
	for _, rt := range ItemRoutes(items) {
		var opts []nicetoken.RequireOption
		if rt.Sensitive {
			opts = append(opts, nicetoken.Sensitive())
		}
		mux.Handle(rt.Method+" "+rt.Pattern, v.Middleware(v.Require(rt.Permission, opts...)(rt.Handler)))
	}
	return httpx.WithCORS(corsOrigins, mux)
}
