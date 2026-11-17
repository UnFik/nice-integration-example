package itemusecase

import (
	"context"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

// DeleteItem returns domain.ErrItemNotFound for an item that is not in the scope.
func (s *Service) DeleteItem(ctx context.Context, sc domain.Scope, id string) error {
	if err := sc.Validate(); err != nil {
		return err
	}
	return s.Repo.Delete(ctx, sc, id)
}
