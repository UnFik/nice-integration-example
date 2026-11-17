package itempostgres

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

func (r *Repository) List(ctx context.Context, s domain.Scope) ([]domain.Item, error) {
	rows, err := r.pool.Query(ctx,
		"SELECT "+columns+" FROM items WHERE org_id = $1 AND project_id = $2 ORDER BY create_time, id", s.OrgID, s.ProjectID)
	if err != nil {
		return nil, err
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Item, error) { return scanItem(row) })
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []domain.Item{}
	}
	return items, nil
}
