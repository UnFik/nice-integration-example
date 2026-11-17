package bootstrap

import (
	"context"
	"fmt"

	itemmemory "github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/memory/item"
	itempostgres "github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/postgres/item"
	itemusecase "github.com/nice-platform/example/sharedservice-oauth/backend/internal/app/item/usecase"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/platform/config"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/platform/postgres"
	"github.com/nice-platform/example/sharedservice-oauth/backend/migrations"
)

// ItemStore is an item repository that can also create the tenant of an
// organization, the provisioning hook of the verifier.
type ItemStore interface {
	itemusecase.Repo
	TenantCreator
}

// TenantCreator creates the tenant of an organization; it must be idempotent.
type TenantCreator interface {
	CreateTenant(ctx context.Context, org string) error
}

// OpenItemStore picks the store by DATABASE_URL: PostgreSQL with its
// migrations applied when set, memory when empty. close releases the store.
// The errors name the variable and never its value.
func OpenItemStore(ctx context.Context, cfg config.Config) (store ItemStore, close func(), err error) {
	if cfg.DatabaseURL == "" {
		return itemmemory.NewRepository(), func() {}, nil
	}
	pool, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: cannot connect to PostgreSQL: %w", config.EnvDatabaseURL, err)
	}
	if err := postgres.Migrate(ctx, pool, migrations.FS); err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("%s: %w", config.EnvDatabaseURL, err)
	}
	return itempostgres.NewRepository(pool), pool.Close, nil
}

// StoreName is the store kind for the start log.
func StoreName(cfg config.Config) string {
	if cfg.DatabaseURL == "" {
		return "memory"
	}
	return "postgres"
}
