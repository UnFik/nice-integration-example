package config

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testKey = "nice_sk_testprefix.testsecretvalue"

func env(vars map[string]string) LookupFunc {
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

// fullEnv is every required variable with a valid value.
func fullEnv() map[string]string {
	return map[string]string{EnvIssuer: "http://localhost:8080", EnvAccountID: "sa_test", EnvAPIKey: testKey}
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

func with(m map[string]string, key, value string) map[string]string {
	out := without(m, key)
	out[key] = value
	return out
}

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := Load(env(fullEnv()), filepath.Join(t.TempDir(), ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.NadiURL != "http://127.0.0.1:8081" || cfg.WatchInterval != 30*time.Second {
		t.Fatalf("defaults = %+v", cfg.LogValue())
	}
}

func TestLoadConfigReadsDotEnvAndTheEnvironmentWins(t *testing.T) {
	dotenv := writeDotEnv(t, "# comment\nNICE_ISSUER=\"http://localhost:8080/\"\nexport NICE_SA_ID=sa_file\nNICE_SA_API_KEY='"+testKey+"'\nNADI_URL=http://nadi.test/\nWATCH_INTERVAL=5s\n")
	cfg, err := Load(env(map[string]string{EnvAccountID: "sa_env"}), dotenv)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Issuer != "http://localhost:8080" || cfg.AccountID != "sa_env" || cfg.APIKey != testKey ||
		cfg.NadiURL != "http://nadi.test" || cfg.WatchInterval != 5*time.Second {
		t.Fatalf("config = %+v", cfg.LogValue())
	}
}

func TestLoadConfigNamesTheMissingVariable(t *testing.T) {
	for _, key := range []string{EnvIssuer, EnvAccountID, EnvAPIKey} {
		_, err := Load(env(without(fullEnv(), key)), filepath.Join(t.TempDir(), ".env"))
		if err == nil || !strings.Contains(err.Error(), key) {
			t.Errorf("%s missing: err = %v", key, err)
		}
		if err != nil && strings.Contains(err.Error(), testKey) {
			t.Errorf("%s missing: the error holds the API key", key)
		}
	}
}

func TestLoadConfigRefusesBadValues(t *testing.T) {
	for key, value := range map[string]string{EnvIssuer: "localhost:8080", EnvNadiURL: "ftp://nadi", EnvWatchInterval: "soon"} {
		_, err := Load(env(with(fullEnv(), key, value)), filepath.Join(t.TempDir(), ".env"))
		if err == nil || !strings.Contains(err.Error(), key) {
			t.Errorf("%s=%s: err = %v", key, value, err)
		}
	}
	if _, err := Load(env(with(fullEnv(), EnvWatchInterval, "-1s")), filepath.Join(t.TempDir(), ".env")); err == nil {
		t.Error("a negative interval was accepted")
	}
}

func TestSecretsStayOutOfErrorsAndLogs(t *testing.T) {
	// A malformed line that holds the key: the error names the line number only.
	_, err := Load(env(nil), writeDotEnv(t, testKey+"\n"))
	if err == nil || strings.Contains(err.Error(), testKey) {
		t.Fatalf("err = %v", err)
	}
	cfg, err := Load(env(fullEnv()), filepath.Join(t.TempDir(), ".env"))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	slog.New(slog.NewTextHandler(&buf, nil)).Info("config", slog.Any("config", cfg))
	if strings.Contains(buf.String(), testKey) || !strings.Contains(buf.String(), "sa_test") {
		t.Fatalf("log line = %s", buf.String())
	}
}
