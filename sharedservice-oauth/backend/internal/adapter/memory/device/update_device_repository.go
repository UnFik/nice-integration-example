package devicememory

import (
	"context"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

func (r *Repository) Update(_ context.Context, s domain.Scope, id string, f domain.DeviceFields) (domain.Device, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := keyOf(s)
	it, ok := r.devices[k][id]
	if !ok {
		return domain.Device{}, domain.ErrDeviceNotFound
	}
	it.Name, it.SerialNumber, it.Status, it.Location = f.Name, f.SerialNumber, f.Status, f.Location
	it.UpdateTime = r.now().UTC()
	r.devices[k][id] = it
	return it, nil
}
