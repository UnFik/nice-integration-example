package devicememory

import (
	"context"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

func (r *Repository) Create(_ context.Context, s domain.Scope, f domain.DeviceFields) (domain.Device, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tenants[s.OrgID] = struct{}{}
	k := keyOf(s)
	if r.devices[k] == nil {
		r.devices[k] = map[string]domain.Device{}
	}
	now := r.now().UTC()
	it := domain.Device{
		ID: newDeviceID(), OrgID: s.OrgID, ProjectID: s.ProjectID, Name: f.Name,
		SerialNumber: f.SerialNumber, Status: f.Status, Location: f.Location,
		CreatedBy: s.Subject, ActedBy: s.Actor, CreateTime: now, UpdateTime: now,
	}
	r.devices[k][it.ID] = it
	return it, nil
}
