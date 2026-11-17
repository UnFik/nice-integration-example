package itemusecase

import (
	"context"

	itemdto "github.com/nice-platform/example/sharedservice-oauth/backend/internal/app/item/dto"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

// CreateItem validates in and stores the item in s. The user (s.Subject) stays
// the creator; an agent that exchanged the user's token is recorded beside it
// as s.Actor.
func (s *Service) CreateItem(ctx context.Context, sc domain.Scope, in itemdto.ItemInput) (domain.Item, error) {
	if err := sc.Validate(); err != nil {
		return domain.Item{}, err
	}
	f, err := domain.NewItemFields(in.Name, in.Quantity)
	if err != nil {
		return domain.Item{}, err
	}
	return s.Repo.Create(ctx, sc, f)
}
