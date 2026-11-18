package devicepostgres

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

// Create also provisions the tenant, so the foreign key holds even when the
// device is written by a process that never ran the provisioning hook.
func (r *Repository) Create(ctx context.Context, s domain.Scope, f domain.DeviceFields) (domain.Device, error) {
	var it domain.Device
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "INSERT INTO tenants (org_id) VALUES ($1) ON CONFLICT DO NOTHING", s.OrgID); err != nil {
			return err
		}
		var err error
		it, err = scanDevice(tx.QueryRow(ctx,
			"INSERT INTO devices (id, org_id, project_id, created_by, acted_by, name, serial_number, status, location) "+
				"VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, $7, $8, $9) RETURNING "+columns,
			newID(), s.OrgID, s.ProjectID, s.Subject, s.Actor, f.Name, f.SerialNumber, f.Status, f.Location))
		return err
	})
	return it, err
}
