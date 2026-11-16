package itemmemory

import "github.com/nice-platform/example/shared-service-oauth/internal/domain"

func (r *Repository) Create(org string, f domain.ItemFields, createdBy, actedBy string) domain.Item {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.tenants[org] == nil {
		r.tenants[org] = map[string]domain.Item{}
	}
	now := r.now().UTC()
	it := domain.Item{ID: newItemID(), Name: f.Name, Quantity: f.Quantity, CreatedBy: createdBy, ActedBy: actedBy, CreateTime: now, UpdateTime: now}
	r.tenants[org][it.ID] = it
	return it
}
