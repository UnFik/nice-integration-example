// Package itemusecase holds the use cases of the item resource.
package itemusecase

import "github.com/nice-platform/example/shared-service-oauth/internal/domain"

// Repo is what the item use cases need from storage. Every method is scoped to
// one organization: the tenant of the verified token.
type Repo interface {
	List(org string) []domain.Item
	Get(org, id string) (domain.Item, bool)
	Create(org string, f domain.ItemFields, createdBy, actedBy string) domain.Item
	Update(org, id string, f domain.ItemFields) (domain.Item, bool)
	Delete(org, id string) bool
}
