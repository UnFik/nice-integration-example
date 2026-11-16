package itemmemory

import "github.com/nice-platform/example/shared-service-oauth/internal/domain"

func (r *Repository) Update(org, id string, f domain.ItemFields) (domain.Item, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	it, ok := r.tenants[org][id]
	if !ok {
		return domain.Item{}, false
	}
	it.Name, it.Quantity, it.UpdateTime = f.Name, f.Quantity, r.now().UTC()
	r.tenants[org][id] = it
	return it, true
}
