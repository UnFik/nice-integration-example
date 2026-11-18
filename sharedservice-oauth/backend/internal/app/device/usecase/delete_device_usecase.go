package deviceusecase

import (
	"context"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

// DeleteDevice returns domain.ErrDeviceNotFound for a device that is not in the scope.
func (s *Service) DeleteDevice(ctx context.Context, sc domain.Scope, id string) error {
	if err := sc.Validate(); err != nil {
		return err
	}
	return s.Repo.Delete(ctx, sc, id)
}
