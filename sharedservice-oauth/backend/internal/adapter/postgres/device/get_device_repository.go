package devicepostgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

func (r *Repository) Get(ctx context.Context, s domain.Scope, id string) (domain.Device, error) {
	key, ok := parseID(id)
	if !ok {
		return domain.Device{}, domain.ErrDeviceNotFound
	}
	it, err := scanDevice(r.pool.QueryRow(ctx,
		"SELECT "+columns+" FROM devices WHERE org_id = $1 AND project_id = $2 AND id = $3", s.OrgID, s.ProjectID, key))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Device{}, domain.ErrDeviceNotFound
	}
	return it, err
}
