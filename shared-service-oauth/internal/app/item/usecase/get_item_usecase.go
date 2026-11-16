package itemusecase

import "github.com/nice-platform/example/shared-service-oauth/internal/domain"

// GetItem returns domain.ErrItemNotFound for an item that is not in the organization.
func (s *Service) GetItem(org, id string) (domain.Item, error) {
	it, ok := s.Repo.Get(org, id)
	if !ok {
		return domain.Item{}, domain.ErrItemNotFound
	}
	return it, nil
}
