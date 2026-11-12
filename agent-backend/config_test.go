package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func env(vars map[string]string) lookupFunc {
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
	return map[string]string{
		envIssuer:        "http://localhost:8080",
		envClientID:      "agen-backend-id",
		envClientSecret:  "secret-from-env",
		envInventarisURL: "http://127.0.0.1:8081",
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
	for _, key := range []string{envIssuer, envClientID, envClientSecret, envInventarisURL} {
		missing := filepath.Join(t.TempDir(), ".env")
		err := run(context.Background(), env(without(fullEnv(), key)), missing, slog.New(slog.DiscardHandler))
		if err == nil || !strings.Contains(err.Error(), key) {
			t.Errorf("run without %s = %v, want an error naming it", key, err)
		}
	}
}

func TestRunStopsWhenTheEnvironmentEmptiesARequiredVariable(t *testing.T) {
	dotenv := writeDotEnv(t, "NICE_CLIENT_SECRET=secret-from-file\n")
	vars := fullEnv()
	vars[envClientSecret] = ""
	err := run(context.Background(), env(vars), dotenv, slog.New(slog.DiscardHandler))
	if err == nil || !strings.Contains(err.Error(), envClientSecret) {
		t.Fatalf("run with an empty secret = %v, want an error naming %s", err, envClientSecret)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := loadConfig(env(fullEnv()), filepath.Join(t.TempDir(), ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Listen != "127.0.0.1:8083" || cfg.PublicURL != "http://127.0.0.1:8083" || cfg.JWKSFile != "data/jwks.json" ||
		cfg.ProxyURL != "http://localhost:8080" || cfg.InventarisURL != "http://127.0.0.1:8081" {
		t.Fatalf("defaults = %+v", cfg)
	}
}

func TestLoadConfigReadsDotEnvAndTheEnvironmentWins(t *testing.T) {
	dotenv := writeDotEnv(t, `# comment
NICE_ISSUER="http://localhost:8080/"
export NICE_CLIENT_ID=from-file
NICE_CLIENT_SECRET='secret-from-file'
INVENTARIS_URL=http://127.0.0.1:9081/
NICE_PROXY_URL=http://127.0.0.1:9080
LISTEN_ADDR=127.0.0.1:9083
`)
	cfg, err := loadConfig(env(map[string]string{envClientSecret: "secret-from-env", envPublicURL: "http://agent.test/"}), dotenv)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Issuer != "http://localhost:8080" || cfg.ClientID != "from-file" || cfg.ClientSecret != "secret-from-env" ||
		cfg.InventarisURL != "http://127.0.0.1:9081" || cfg.ProxyURL != "http://127.0.0.1:9080" ||
		cfg.Listen != "127.0.0.1:9083" || cfg.PublicURL != "http://agent.test" {
		t.Fatalf("config = %+v", cfg)
	}
}

func TestLoadConfigRefusesURLsThatAreNotURLs(t *testing.T) {
	for _, key := range []string{envIssuer, envInventarisURL, envProxyURL, envPublicURL} {
		for _, bad := range []string{"localhost:8080", "ftp://nice", "http://"} {
			vars := fullEnv()
			vars[key] = bad
			_, err := loadConfig(env(vars), filepath.Join(t.TempDir(), ".env"))
			if err == nil || !strings.Contains(err.Error(), key) {
				t.Errorf("%s=%q: err = %v, want an error naming %s", key, bad, err, key)
			}
		}
	}
}

func TestConfigErrorsAndLogsNeverShowTheSecret(t *testing.T) {
	vars := fullEnv()
	vars[envIssuer] = "not a url"
	_, err := loadConfig(env(vars), filepath.Join(t.TempDir(), ".env"))
	if err == nil || strings.Contains(err.Error(), "secret-from-env") {
		t.Fatalf("err = %v", err)
	}
	cfg, err := loadConfig(env(fullEnv()), filepath.Join(t.TempDir(), ".env"))
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	slog.New(slog.NewTextHandler(&b, nil)).Info("config", slog.Any("config", cfg))
	if strings.Contains(b.String(), "secret-from-env") {
		t.Fatalf("the logged config shows the secret: %s", b.String())
	}
}

func TestEnvExampleListsEveryVariableWithoutSecrets(t *testing.T) {
	file, err := readDotEnv(".env.example")
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{envIssuer, envClientID, envClientSecret, envInventarisURL, envProxyURL, envListen, envPublicURL, envJWKSFile} {
		if _, ok := file[k]; !ok {
			t.Errorf(".env.example does not list %s", k)
		}
	}
	if file[envClientSecret] != "" || file[envClientID] != "" {
		t.Errorf(".env.example holds a client id or secret")
	}
	cfg, err := loadConfig(env(map[string]string{envClientID: "id", envClientSecret: "s"}), ".env.example")
	if err != nil {
		t.Fatalf(".env.example as .env with the client filled: %v", err)
	}
	if cfg.Listen != defaultListen || cfg.InventarisURL != "http://127.0.0.1:8081" {
		t.Fatalf(".env.example gives %+v", cfg)
	}
}

func TestLoadConfigRefusesAMalformedDotEnv(t *testing.T) {
	_, err := loadConfig(env(nil), writeDotEnv(t, "NICE_ISSUER\n"))
	if err == nil || !strings.Contains(err.Error(), "line 1") {
		t.Fatalf("err = %v, want the bad line", err)
	}
}
