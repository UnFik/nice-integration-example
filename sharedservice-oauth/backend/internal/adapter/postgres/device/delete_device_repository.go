package devicepostgres

import (
	"context"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

func (r *Repository) Delete(ctx context.Context, s domain.Scope, id string) error {
	key, ok := parseID(id)
	if !ok {
		return domain.ErrDeviceNotFound
	}
	tag, err := r.pool.Exec(ctx, "DELETE FROM devices WHERE org_id = $1 AND project_id = $2 AND id = $3", s.OrgID, s.ProjectID, key)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrDeviceNotFound
	}
	return nil
}
