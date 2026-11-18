package devicememory

import (
	"context"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

func (r *Repository) Get(_ context.Context, s domain.Scope, id string) (domain.Device, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	it, ok := r.devices[keyOf(s)][id]
	if !ok {
		return domain.Device{}, domain.ErrDeviceNotFound
	}
	return it, nil
}
