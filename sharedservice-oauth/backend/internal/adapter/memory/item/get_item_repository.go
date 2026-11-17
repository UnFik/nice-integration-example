package itemmemory

import (
	"context"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

func (r *Repository) Get(_ context.Context, s domain.Scope, id string) (domain.Item, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	it, ok := r.items[keyOf(s)][id]
	if !ok {
		return domain.Item{}, domain.ErrItemNotFound
	}
	return it, nil
}
