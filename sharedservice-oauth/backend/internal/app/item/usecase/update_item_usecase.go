package itemusecase

import (
	"context"

	itemdto "github.com/nice-platform/example/sharedservice-oauth/backend/internal/app/item/dto"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

// UpdateItem replaces the name and quantity of an item with the same rules as creating one.
func (s *Service) UpdateItem(ctx context.Context, sc domain.Scope, id string, in itemdto.ItemInput) (domain.Item, error) {
	if err := sc.Validate(); err != nil {
		return domain.Item{}, err
	}
	f, err := domain.NewItemFields(in.Name, in.Quantity)
	if err != nil {
		return domain.Item{}, err
	}
	return s.Repo.Update(ctx, sc, id, f)
}
