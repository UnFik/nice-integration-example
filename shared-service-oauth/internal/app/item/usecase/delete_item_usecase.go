package itemusecase

import "github.com/nice-platform/example/shared-service-oauth/internal/domain"

// DeleteItem returns domain.ErrItemNotFound for an item that is not in the organization.
func (s *Service) DeleteItem(org, id string) error {
	if !s.Repo.Delete(org, id) {
		return domain.ErrItemNotFound
	}
	return nil
}
