package devicememory

import (
	"cmp"
	"context"
	"slices"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

func (r *Repository) List(_ context.Context, s domain.Scope) ([]domain.Device, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	in := r.devices[keyOf(s)]
	out := make([]domain.Device, 0, len(in))
	for _, it := range in {
		out = append(out, it)
	}
	slices.SortFunc(out, func(a, b domain.Device) int {
		return cmp.Or(a.CreateTime.Compare(b.CreateTime), cmp.Compare(a.ID, b.ID))
	})
	return out, nil
}
