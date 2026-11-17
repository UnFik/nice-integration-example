package itempostgres

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

// Create also provisions the tenant, so the foreign key holds even when the
// item is written by a process that never ran the provisioning hook.
func (r *Repository) Create(ctx context.Context, s domain.Scope, f domain.ItemFields) (domain.Item, error) {
	var it domain.Item
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "INSERT INTO tenants (org_id) VALUES ($1) ON CONFLICT DO NOTHING", s.OrgID); err != nil {
			return err
		}
		var err error
		it, err = scanItem(tx.QueryRow(ctx,
			"INSERT INTO items (id, org_id, project_id, created_by, acted_by, name, quantity) "+
				"VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, $7) RETURNING "+columns,
			newID(), s.OrgID, s.ProjectID, s.Subject, s.Actor, f.Name, f.Quantity))
		return err
	})
	return it, err
}
