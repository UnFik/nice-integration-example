// Package deviceusecase holds the use cases of the device resource.
package deviceusecase

import (
	"context"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

// Repo is what the device use cases need from storage. Every method is scoped by
// s, which the caller builds from the verified token; the repository has no
// other way to name an organization or a project. Get, Update and Delete
// return domain.ErrDeviceNotFound for an id outside s.
type Repo interface {
	List(ctx context.Context, s domain.Scope) ([]domain.Device, error)
	Get(ctx context.Context, s domain.Scope, id string) (domain.Device, error)
	Create(ctx context.Context, s domain.Scope, f domain.DeviceFields) (domain.Device, error)
	Update(ctx context.Context, s domain.Scope, id string, f domain.DeviceFields) (domain.Device, error)
	Delete(ctx context.Context, s domain.Scope, id string) error
}
