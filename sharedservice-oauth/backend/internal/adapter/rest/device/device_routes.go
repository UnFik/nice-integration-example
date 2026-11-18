// Package device serves the REST API of the device resource.
package device

import (
	"net/http"

	deviceusecase "github.com/nice-platform/example/sharedservice-oauth/backend/internal/app/device/usecase"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

// Handler serves the device operations.
type Handler struct {
	Devices *deviceusecase.Service
}

func New(devices *deviceusecase.Service) *Handler { return &Handler{Devices: devices} }

// Route is one device operation. The same table registers the routes and is
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
		{http.MethodGet, "/devices", domain.PermRead, false, h.ListDevices},
		{http.MethodPost, "/devices", domain.PermCreate, false, h.CreateDevice},
		{http.MethodGet, "/devices/{id}", domain.PermRead, false, h.GetDevice},
		{http.MethodPut, "/devices/{id}", domain.PermEdit, false, h.UpdateDevice},
		{http.MethodDelete, "/devices/{id}", domain.PermDelete, true, h.DeleteDevice},
	}
}
