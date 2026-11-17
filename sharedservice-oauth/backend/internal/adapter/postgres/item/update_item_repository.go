package itempostgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

func (r *Repository) Update(ctx context.Context, s domain.Scope, id string, f domain.ItemFields) (domain.Item, error) {
	key, ok := parseID(id)
	if !ok {
		return domain.Item{}, domain.ErrItemNotFound
	}
	it, err := scanItem(r.pool.QueryRow(ctx,
		"UPDATE items SET name = $4, quantity = $5, update_time = now() "+
			"WHERE org_id = $1 AND project_id = $2 AND id = $3 RETURNING "+columns,
		s.OrgID, s.ProjectID, key, f.Name, f.Quantity))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Item{}, domain.ErrItemNotFound
	}
	return it, err
}
