package bootstrap

import (
	"context"
	"log/slog"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/platform/config"
	"github.com/nice-platform/nice/pkg/nicetoken"
)

// VerifierConfig wires nicetoken for this service. Tests reuse it and only swap
// the key source and the checker.
func VerifierConfig(cfg config.Config, tenants TenantCreator, log *slog.Logger) nicetoken.Config {
	c := nicetoken.Config{
		Issuer:   cfg.Issuer,
		Service:  domain.ServiceSlug,
		JWKSFile: cfg.JWKSFile,
		// A valid token for this service already proves that NICE entitled the
		// organization, so its first request is a safe moment to create the tenant.
		Tenants: nicetoken.ProvisionOnFirstUse(tenants.CreateTenant),
		Log:     log,
	}
	if cfg.ServiceAPIKey != "" {
		// Without a checker, nicetoken refuses sensitive operations with 503 (fail closed).
		c.Checker = nicetoken.NewHTTPChecker(cfg.Issuer, cfg.ServiceAPIKey)
	}
	return c
}

// StartFeed reads the access change feed when the nice.service key is set, so
// tokens issued before a revocation are refused with token_stale.
func StartFeed(ctx context.Context, v *nicetoken.Verifier, cfg config.Config, log *slog.Logger) error {
	if cfg.ServiceAPIKey == "" {
		log.Warn(config.EnvServiceAPIKey + " is not set: the access change feed is off, so revoked access is refused only when the token expires (up to 30 minutes), and DELETE /devices/{id} answers 503")
		return nil
	}
	return v.StartFeed(ctx, nicetoken.FeedConfig{APIKey: cfg.ServiceAPIKey})
}
