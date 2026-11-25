package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	return map[string]string{
		EnvIssuer:       "http://localhost:8080",
		EnvClientID:     "agen-backend-id",
		EnvClientSecret: "secret-from-env",
		EnvNadiURL:      "http://127.0.0.1:8081",
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

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := Load(env(fullEnv()), filepath.Join(t.TempDir(), ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Listen != "127.0.0.1:8083" || cfg.PublicURL != "http://127.0.0.1:8083" || cfg.JWKSFile != "data/jwks.json" ||
		cfg.NadiURL != "http://127.0.0.1:8081" {
		t.Fatalf("defaults = %+v", cfg)
	}
}

func TestLoadConfigReadsDotEnvAndTheEnvironmentWins(t *testing.T) {
	dotenv := writeDotEnv(t, `# comment
NICE_ISSUER="http://localhost:8080/"
export NICE_CLIENT_ID=from-file
NICE_CLIENT_SECRET='secret-from-file'
NADI_URL=http://127.0.0.1:9081/
LISTEN_ADDR=127.0.0.1:9083
`)
	cfg, err := Load(env(map[string]string{EnvClientSecret: "secret-from-env", EnvPublicURL: "http://agent.test/"}), dotenv)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Issuer != "http://localhost:8080" || cfg.ClientID != "from-file" || cfg.ClientSecret != "secret-from-env" ||
		cfg.NadiURL != "http://127.0.0.1:9081" ||
		cfg.Listen != "127.0.0.1:9083" || cfg.PublicURL != "http://agent.test" {
		t.Fatalf("config = %+v", cfg)
	}
}

func TestLoadConfigRefusesURLsThatAreNotURLs(t *testing.T) {
	for _, key := range []string{EnvIssuer, EnvNadiURL, EnvPublicURL} {
		for _, bad := range []string{"localhost:8080", "ftp://nice", "http://"} {
			vars := fullEnv()
			vars[key] = bad
			_, err := Load(env(vars), filepath.Join(t.TempDir(), ".env"))
			if err == nil || !strings.Contains(err.Error(), key) {
				t.Errorf("%s=%q: err = %v, want an error naming %s", key, bad, err, key)
			}
		}
	}
}

func TestConfigErrorsAndLogsNeverShowTheSecret(t *testing.T) {
	vars := fullEnv()
	vars[EnvIssuer] = "not a url"
	_, err := Load(env(vars), filepath.Join(t.TempDir(), ".env"))
	if err == nil || strings.Contains(err.Error(), "secret-from-env") {
		t.Fatalf("err = %v", err)
	}
	cfg, err := Load(env(fullEnv()), filepath.Join(t.TempDir(), ".env"))
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
	file, err := readDotEnv("../../../.env.example")
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{EnvIssuer, EnvClientID, EnvClientSecret, EnvNadiURL, EnvListen, EnvPublicURL, EnvJWKSFile} {
		if _, ok := file[k]; !ok {
			t.Errorf(".env.example does not list %s", k)
		}
	}
	if file[EnvClientSecret] != "" || file[EnvClientID] != "" {
		t.Errorf(".env.example holds a client id or secret")
	}
	// The agent holds no credential of NICE besides its service client.
	if _, ok := file["NICE_SERVICE_API_KEY"]; ok {
		t.Errorf(".env.example still lists NICE_SERVICE_API_KEY")
	}
	cfg, err := Load(env(map[string]string{EnvClientID: "id", EnvClientSecret: "s"}), "../../../.env.example")
	if err != nil {
		t.Fatalf(".env.example as .env with the client filled: %v", err)
	}
	if cfg.Listen != defaultListen || cfg.NadiURL != "http://127.0.0.1:8081" {
		t.Fatalf(".env.example gives %+v", cfg)
	}
}

func TestLoadConfigRefusesAMalformedDotEnv(t *testing.T) {
	_, err := Load(env(nil), writeDotEnv(t, "NICE_ISSUER\n"))
	if err == nil || !strings.Contains(err.Error(), "line 1") {
		t.Fatalf("err = %v, want the bad line", err)
	}
}
