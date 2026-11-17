package itemusecase

import (
	"context"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

// ListItems returns the items of the project of the scope, oldest first.
func (s *Service) ListItems(ctx context.Context, sc domain.Scope) ([]domain.Item, error) {
	if err := sc.Validate(); err != nil {
		return nil, err
	}
	return s.Repo.List(ctx, sc)
}
