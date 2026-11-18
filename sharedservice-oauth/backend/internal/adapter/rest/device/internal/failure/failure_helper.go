// Package failure maps the errors of the device use cases to HTTP responses.
package failure

import (
	"errors"
	"net/http"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/internal/reply"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

func Write(w http.ResponseWriter, err error) {
	var bad *domain.InvalidFieldError
	switch {
	case errors.Is(err, domain.ErrDeviceNotFound):
		reply.Error(w, http.StatusNotFound, "not_found", "")
	case errors.As(err, &bad):
		reply.Error(w, http.StatusBadRequest, "invalid_argument", bad.Field)
	case errors.Is(err, domain.ErrProjectRequired):
		reply.Error(w, http.StatusForbidden, "insufficient_scope", reply.ProjectRequired)
	case errors.Is(err, domain.ErrOrgRequired):
		reply.Error(w, http.StatusServiceUnavailable, "temporarily_unavailable", "tenant_unavailable")
	default:
		reply.Error(w, http.StatusInternalServerError, "server_error", "")
	}
}
