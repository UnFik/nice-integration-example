// Package itempostgres keeps items in PostgreSQL. Every query filters by the
// org_id and project_id of the scope, so an id from another scope is not found.
package itempostgres

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

const itemPrefix = "itm_"

// columns is the select list scanItem reads.
const columns = "id, org_id, project_id, name, quantity, created_by, COALESCE(acted_by, ''), create_time, update_time"

// CreateTenant is the provisioning hook of nicetoken.ProvisionOnFirstUse.
// ON CONFLICT makes it idempotent and safe for two first requests at once.
func (r *Repository) CreateTenant(ctx context.Context, org string) error {
	_, err := r.pool.Exec(ctx, "INSERT INTO tenants (org_id) VALUES ($1) ON CONFLICT DO NOTHING", org)
	return err
}

func (r *Repository) HasTenant(ctx context.Context, org string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM tenants WHERE org_id = $1)", org).Scan(&ok)
	return ok, err
}

// The API shows the uuid key as itm_<32 hex>, the same shape the memory
// adapter uses, so clients do not see which store runs.
func formatID(id [16]byte) string { return itemPrefix + hex.EncodeToString(id[:]) }

func parseID(s string) ([16]byte, bool) {
	var id [16]byte
	raw, ok := strings.CutPrefix(s, itemPrefix)
	if !ok || len(raw) != 32 {
		return id, false
	}
	if _, err := hex.Decode(id[:], []byte(raw)); err != nil {
		return id, false
	}
	return id, true
}

func newID() [16]byte {
	var id [16]byte
	_, _ = rand.Read(id[:])
	id[6] = id[6]&0x0f | 0x40
	id[8] = id[8]&0x3f | 0x80
	return id
}

func scanItem(row pgx.Row) (domain.Item, error) {
	var (
		it domain.Item
		id [16]byte
	)
	err := row.Scan(&id, &it.OrgID, &it.ProjectID, &it.Name, &it.Quantity, &it.CreatedBy, &it.ActedBy, &it.CreateTime, &it.UpdateTime)
	if err != nil {
		return domain.Item{}, err
	}
	it.ID = formatID(id)
	it.CreateTime, it.UpdateTime = it.CreateTime.UTC(), it.UpdateTime.UTC()
	return it, nil
}
