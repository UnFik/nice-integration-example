package devicepostgres

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

func (r *Repository) List(ctx context.Context, s domain.Scope) ([]domain.Device, error) {
	rows, err := r.pool.Query(ctx,
		"SELECT "+columns+" FROM devices WHERE org_id = $1 AND project_id = $2 ORDER BY create_time, id", s.OrgID, s.ProjectID)
	if err != nil {
		return nil, err
	}
	devices, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Device, error) { return scanDevice(row) })
	if err != nil {
		return nil, err
	}
	if devices == nil {
		devices = []domain.Device{}
	}
	return devices, nil
}
