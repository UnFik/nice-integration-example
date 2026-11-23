package bootstrap

import (
	"log/slog"
	"testing"

	"github.com/ABA-Developer/nice-auth-sdk/verify"
	"github.com/nice-platform/example/agent-backend/internal/platform/config"
)

func TestTheVerifierResolvesPermissionReferencesOnlyWithTheServiceKey(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	if c := VerifierConfig(config.Config{Issuer: "http://localhost:8080"}, log); c.Resolver != nil {
		t.Fatalf("resolver without %s = %T, want nil", config.EnvServiceAPIKey, c.Resolver)
	}
	c := VerifierConfig(config.Config{Issuer: "http://localhost:8080", ServiceAPIKey: "nice_sk_test.secret"}, log)
	if _, ok := c.Resolver.(*verify.HTTPResolver); !ok {
		t.Fatalf("resolver with %s = %T, want *verify.HTTPResolver", config.EnvServiceAPIKey, c.Resolver)
	}
}
