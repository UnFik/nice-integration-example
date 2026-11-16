package itemusecase

import "github.com/nice-platform/example/shared-service-oauth/internal/domain"

// ListItems returns the items of the organization, oldest first.
func (s *Service) ListItems(org string) []domain.Item {
	return s.Repo.List(org)
}
