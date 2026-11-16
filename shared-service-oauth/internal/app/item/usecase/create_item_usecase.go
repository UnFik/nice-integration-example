package itemusecase

import (
	itemdto "github.com/nice-platform/example/shared-service-oauth/internal/app/item/dto"
	"github.com/nice-platform/example/shared-service-oauth/internal/domain"
)

// CreateItem validates in and stores the item. The user stays the creator; an
// agent that exchanged the user's token is recorded beside it as actedBy.
func (s *Service) CreateItem(org, createdBy, actedBy string, in itemdto.ItemInput) (domain.Item, error) {
	f, err := domain.NewItemFields(in.Name, in.Quantity)
	if err != nil {
		return domain.Item{}, err
	}
	return s.Repo.Create(org, f, createdBy, actedBy), nil
}
