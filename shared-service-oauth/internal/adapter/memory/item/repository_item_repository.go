// Package itemmemory keeps items in memory; they are lost on restart.
package itemmemory

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/nice-platform/example/shared-service-oauth/internal/domain"
)

// Repository keeps the items of every tenant. The tenant key is always the
// organization of the verified token.
type Repository struct {
	mu      sync.Mutex
	tenants map[string]map[string]domain.Item
	now     func() time.Time
}

func NewRepository() *Repository {
	return &Repository{tenants: map[string]map[string]domain.Item{}, now: time.Now}
}

// CreateTenant is the provisioning hook of nicetoken.ProvisionOnFirstUse, so it
// must be idempotent.
func (r *Repository) CreateTenant(_ context.Context, org string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tenants[org]; !ok {
		r.tenants[org] = map[string]domain.Item{}
	}
	return nil
}

func (r *Repository) HasTenant(org string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.tenants[org]
	return ok
}

func newItemID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "itm_" + hex.EncodeToString(b)
}
