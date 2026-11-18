package deviceusecase

import (
	"context"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

// ListDevices returns the devices of the project of the scope, oldest first.
func (s *Service) ListDevices(ctx context.Context, sc domain.Scope) ([]domain.Device, error) {
	if err := sc.Validate(); err != nil {
		return nil, err
	}
	return s.Repo.List(ctx, sc)
}
