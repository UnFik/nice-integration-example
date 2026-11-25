package bootstrap

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nice-platform/example/agent-backend/internal/platform/config"
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

func fullEnv() map[string]string {
	return map[string]string{
		config.EnvIssuer:       "http://localhost:8080",
		config.EnvClientID:     "agen-backend-id",
		config.EnvClientSecret: "secret-from-env",
	}
}

func without(m map[string]string, key string) map[string]string {
	out := map[string]string{}
	for k, v := range m {
		if k != key {
			out[k] = v
		}
	}
	return out
}

func TestRunStopsAndNamesEachMissingVariable(t *testing.T) {
	for _, key := range []string{config.EnvIssuer, config.EnvClientID, config.EnvClientSecret} {
		missing := filepath.Join(t.TempDir(), ".env")
		err := Run(context.Background(), env(without(fullEnv(), key)), missing, slog.New(slog.DiscardHandler))
		if err == nil || !strings.Contains(err.Error(), key) {
			t.Errorf("Run without %s = %v, want an error naming it", key, err)
		}
	}
}

func TestRunStopsWhenTheEnvironmentEmptiesARequiredVariable(t *testing.T) {
	dotenv := writeDotEnv(t, "NICE_CLIENT_SECRET=secret-from-file\n")
	vars := fullEnv()
	vars[config.EnvClientSecret] = ""
	err := Run(context.Background(), env(vars), dotenv, slog.New(slog.DiscardHandler))
	if err == nil || !strings.Contains(err.Error(), config.EnvClientSecret) {
		t.Fatalf("Run with an empty secret = %v, want an error naming %s", err, config.EnvClientSecret)
	}
}
