package itemmemory

import (
	"cmp"
	"slices"

	"github.com/nice-platform/example/shared-service-oauth/internal/domain"
)

func (r *Repository) List(org string) []domain.Item {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.Item, 0, len(r.tenants[org]))
	for _, it := range r.tenants[org] {
		out = append(out, it)
	}
	slices.SortFunc(out, func(a, b domain.Item) int {
		return cmp.Or(a.CreateTime.Compare(b.CreateTime), cmp.Compare(a.ID, b.ID))
	})
	return out
}
