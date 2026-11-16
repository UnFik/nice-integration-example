package itemmemory

import "github.com/nice-platform/example/shared-service-oauth/internal/domain"

func (r *Repository) Get(org, id string) (domain.Item, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	it, ok := r.tenants[org][id]
	return it, ok
}
