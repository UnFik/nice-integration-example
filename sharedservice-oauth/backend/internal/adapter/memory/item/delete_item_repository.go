package itemmemory

import (
	"context"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

func (r *Repository) Delete(_ context.Context, s domain.Scope, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := keyOf(s)
	if _, ok := r.items[k][id]; !ok {
		return domain.ErrItemNotFound
	}
	delete(r.items[k], id)
	return nil
}
