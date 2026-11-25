package bootstrap

import (
	"context"
	"log/slog"

	"github.com/ABA-Developer/nice-auth-sdk/verify"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/platform/config"
)

// VerifierConfig wires niceauth/verify for this service. Tests reuse it and only swap
// the key source and the checker.
func VerifierConfig(cfg config.Config, tenants TenantCreator, log *slog.Logger) verify.Config {
	return verify.Config{
		Issuer:   cfg.Issuer,
		Service:  domain.ServiceSlug,
		JWKSFile: cfg.JWKSFile,
		// A valid token for this service already proves that NICE entitled the
		// organization, so its first request is a safe moment to create the tenant.
		Tenants: verify.ProvisionOnFirstUse(tenants.CreateTenant),
		// The online check of DELETE /devices/{id} and the resolution of perms_ref forward the token of the request:
		// NICE reads the caller from it, so the service holds no credential of NICE.
		Checker:  verify.NewHTTPChecker(cfg.Issuer),
		Resolver: verify.NewHTTPResolver(cfg.Issuer),
		Log:      log,
	}
}

// StartFeed reads the access change feed, so tokens issued before a revocation
// are refused with token_stale. The feed needs no credential.
func StartFeed(ctx context.Context, v *verify.Verifier) error {
	return v.StartFeed(ctx, verify.FeedConfig{})
}
