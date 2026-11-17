package itemusecase

import (
	"context"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

// GetItem returns domain.ErrItemNotFound for an item that is not in the scope.
func (s *Service) GetItem(ctx context.Context, sc domain.Scope, id string) (domain.Item, error) {
	if err := sc.Validate(); err != nil {
		return domain.Item{}, err
	}
	return s.Repo.Get(ctx, sc, id)
}
