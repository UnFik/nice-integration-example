// Package item serves the REST API of the item resource.
package item

import (
	"net/http"

	itemusecase "github.com/nice-platform/example/shared-service-oauth/internal/app/item/usecase"
	"github.com/nice-platform/example/shared-service-oauth/internal/domain"
)

// Handler serves the item operations.
type Handler struct {
	Items *itemusecase.Service
}

func New(items *itemusecase.Service) *Handler { return &Handler{Items: items} }

// Route is one item operation. The same table registers the routes and is
// compared with the OpenAPI document in the tests, so the permission the
// service enforces cannot drift from the one NICE imports.
type Route struct {
	Method     string
	Pattern    string
	Permission string
	Sensitive  bool
	Handler    http.HandlerFunc
}

func (h *Handler) Routes() []Route {
	return []Route{
		{http.MethodGet, "/items", domain.PermRead, false, h.ListItems},
		{http.MethodPost, "/items", domain.PermCreate, false, h.CreateItem},
		{http.MethodGet, "/items/{id}", domain.PermRead, false, h.GetItem},
		{http.MethodPut, "/items/{id}", domain.PermEdit, false, h.UpdateItem},
		{http.MethodDelete, "/items/{id}", domain.PermDelete, true, h.DeleteItem},
	}
}
