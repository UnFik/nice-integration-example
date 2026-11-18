package deviceusecase

import (
	"context"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

// GetDevice returns domain.ErrDeviceNotFound for a device that is not in the scope.
func (s *Service) GetDevice(ctx context.Context, sc domain.Scope, id string) (domain.Device, error) {
	if err := sc.Validate(); err != nil {
		return domain.Device{}, err
	}
	return s.Repo.Get(ctx, sc, id)
}
