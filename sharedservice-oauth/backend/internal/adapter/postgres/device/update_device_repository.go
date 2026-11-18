package devicepostgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

func (r *Repository) Update(ctx context.Context, s domain.Scope, id string, f domain.DeviceFields) (domain.Device, error) {
	key, ok := parseID(id)
	if !ok {
		return domain.Device{}, domain.ErrDeviceNotFound
	}
	it, err := scanDevice(r.pool.QueryRow(ctx,
		"UPDATE devices SET name = $4, serial_number = $5, status = $6, location = $7, update_time = now() "+
			"WHERE org_id = $1 AND project_id = $2 AND id = $3 RETURNING "+columns,
		s.OrgID, s.ProjectID, key, f.Name, f.SerialNumber, f.Status, f.Location))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Device{}, domain.ErrDeviceNotFound
	}
	return it, err
}
