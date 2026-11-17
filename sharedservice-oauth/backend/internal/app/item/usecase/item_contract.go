// Package itemusecase holds the use cases of the item resource.
package itemusecase

import (
	"context"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

// Repo is what the item use cases need from storage. Every method is scoped by
// s, which the caller builds from the verified token; the repository has no
// other way to name an organization or a project. Get, Update and Delete
// return domain.ErrItemNotFound for an id outside s.
type Repo interface {
	List(ctx context.Context, s domain.Scope) ([]domain.Item, error)
	Get(ctx context.Context, s domain.Scope, id string) (domain.Item, error)
	Create(ctx context.Context, s domain.Scope, f domain.ItemFields) (domain.Item, error)
	Update(ctx context.Context, s domain.Scope, id string, f domain.ItemFields) (domain.Item, error)
	Delete(ctx context.Context, s domain.Scope, id string) error
}
