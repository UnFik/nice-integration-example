package bootstrap

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/platform/config"
)

func env(vars map[string]string) config.LookupFunc {
	return func(k string) (string, bool) {
		v, ok := vars[k]
		return v, ok
	}
}

func writeDotEnv(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRunStopsWithoutIssuer(t *testing.T) {
	missing := filepath.Join(t.TempDir(), ".env")
	err := Run(context.Background(), env(nil), missing, slog.New(slog.DiscardHandler))
	if err == nil || !strings.Contains(err.Error(), "NICE_ISSUER") {
		t.Fatalf("run without issuer = %v, want an error naming NICE_ISSUER", err)
	}
}

func TestRunStopsWhenTheEnvironmentEmptiesTheIssuer(t *testing.T) {
	dotenv := writeDotEnv(t, "NICE_ISSUER=http://localhost:8080\n")
	err := Run(context.Background(), env(map[string]string{config.EnvIssuer: ""}), dotenv, slog.New(slog.DiscardHandler))
	if err == nil || !strings.Contains(err.Error(), "NICE_ISSUER") {
		t.Fatalf("run with an empty issuer = %v, want an error naming NICE_ISSUER", err)
	}
}
