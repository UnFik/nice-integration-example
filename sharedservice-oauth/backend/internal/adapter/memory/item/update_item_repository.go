package itemmemory

import (
	"context"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

func (r *Repository) Update(_ context.Context, s domain.Scope, id string, f domain.ItemFields) (domain.Item, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := keyOf(s)
	it, ok := r.items[k][id]
	if !ok {
		return domain.Item{}, domain.ErrItemNotFound
	}
	it.Name, it.Quantity, it.UpdateTime = f.Name, f.Quantity, r.now().UTC()
	r.items[k][id] = it
	return it, nil
}
