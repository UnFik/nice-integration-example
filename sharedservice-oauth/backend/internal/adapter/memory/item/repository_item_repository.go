// Package itemmemory keeps items in memory; they are lost on restart.
package itemmemory

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

// key is the part of a domain.Scope that owns items.
type key struct{ org, project string }

func keyOf(s domain.Scope) key { return key{s.OrgID, s.ProjectID} }

// Repository keeps the items of every organization and project. It follows the
// same scope rules as the PostgreSQL adapter, so the service behaves the same
// without a database.
type Repository struct {
	mu      sync.Mutex
	tenants map[string]struct{}
	items   map[key]map[string]domain.Item
	now     func() time.Time
}

func NewRepository() *Repository {
	return &Repository{tenants: map[string]struct{}{}, items: map[key]map[string]domain.Item{}, now: time.Now}
}

// CreateTenant is the provisioning hook of nicetoken.ProvisionOnFirstUse, so it
// must be idempotent.
func (r *Repository) CreateTenant(_ context.Context, org string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tenants[org] = struct{}{}
	return nil
}

func (r *Repository) HasTenant(_ context.Context, org string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.tenants[org]
	return ok, nil
}

func newItemID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "itm_" + hex.EncodeToString(b)
}
