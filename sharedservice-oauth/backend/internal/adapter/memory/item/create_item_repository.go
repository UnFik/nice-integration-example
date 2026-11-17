package itemmemory

import (
	"context"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

func (r *Repository) Create(_ context.Context, s domain.Scope, f domain.ItemFields) (domain.Item, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tenants[s.OrgID] = struct{}{}
	k := keyOf(s)
	if r.items[k] == nil {
		r.items[k] = map[string]domain.Item{}
	}
	now := r.now().UTC()
	it := domain.Item{
		ID: newItemID(), OrgID: s.OrgID, ProjectID: s.ProjectID, Name: f.Name, Quantity: f.Quantity,
		CreatedBy: s.Subject, ActedBy: s.Actor, CreateTime: now, UpdateTime: now,
	}
	r.items[k][it.ID] = it
	return it, nil
}
