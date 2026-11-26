package bootstrap

import (
	"log/slog"
	"testing"

	"github.com/ABA-Developer/nice-auth-sdk/verify"
	"github.com/nice-platform/example/agent/backend/internal/platform/config"
)

func TestTheVerifierResolvesPermissionReferencesWithoutConfiguration(t *testing.T) {
	c := VerifierConfig(config.Config{Issuer: "http://localhost:8080"}, slog.New(slog.DiscardHandler))
	if _, ok := c.Resolver.(*verify.HTTPResolver); !ok {
		t.Fatalf("resolver = %T, want *verify.HTTPResolver", c.Resolver)
	}
}
