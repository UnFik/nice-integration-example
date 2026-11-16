package itemusecase

import (
	itemdto "github.com/nice-platform/example/shared-service-oauth/internal/app/item/dto"
	"github.com/nice-platform/example/shared-service-oauth/internal/domain"
)

// UpdateItem replaces the name and quantity of an item with the same rules as creating one.
func (s *Service) UpdateItem(org, id string, in itemdto.ItemInput) (domain.Item, error) {
	f, err := domain.NewItemFields(in.Name, in.Quantity)
	if err != nil {
		return domain.Item{}, err
	}
	it, ok := s.Repo.Update(org, id, f)
	if !ok {
		return domain.Item{}, domain.ErrItemNotFound
	}
	return it, nil
}
