package config

import (
	"os"
	"path/filepath"
	"slices"
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

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := Load(env(map[string]string{EnvIssuer: "http://localhost:8080"}), filepath.Join(t.TempDir(), ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Listen != "127.0.0.1:8081" || cfg.JWKSFile != "data/jwks.json" || cfg.ServiceAPIKey != "" ||
		!slices.Equal(cfg.CORSOrigins, []string{"http://localhost:5180"}) {
		t.Fatalf("defaults = %+v", cfg)
	}
}

func TestLoadConfigReadsDotEnvAndTheEnvironmentWins(t *testing.T) {
	dotenv := writeDotEnv(t, `# comment
NICE_ISSUER="http://localhost:8080"
export LISTEN_ADDR=127.0.0.1:9000

CORS_ALLOWED_ORIGINS= http://localhost:5180 , http://127.0.0.1:5180,
NICE_SERVICE_API_KEY='nice_sk_from_file'
JWKS_FILE=/tmp/from-file.json
`)
	cfg, err := Load(env(map[string]string{EnvJWKSFile: "/tmp/from-env.json"}), dotenv)
	if err != nil {
		t.Fatal(err)
	}
	want := Config{
		Issuer: "http://localhost:8080", Listen: "127.0.0.1:9000", JWKSFile: "/tmp/from-env.json",
		CORSOrigins: []string{"http://localhost:5180", "http://127.0.0.1:5180"}, ServiceAPIKey: "nice_sk_from_file",
	}
	if cfg.Issuer != want.Issuer || cfg.Listen != want.Listen || cfg.JWKSFile != want.JWKSFile ||
		cfg.ServiceAPIKey != want.ServiceAPIKey || !slices.Equal(cfg.CORSOrigins, want.CORSOrigins) {
		t.Fatalf("config = %+v, want %+v", cfg, want)
	}
}

func TestLoadConfigRefusesAnIssuerThatIsNotAURL(t *testing.T) {
	for _, iss := range []string{"localhost:8080", "ftp://nice", "http://"} {
		_, err := Load(env(map[string]string{EnvIssuer: iss}), filepath.Join(t.TempDir(), ".env"))
		if err == nil || !strings.Contains(err.Error(), "NICE_ISSUER") {
			t.Errorf("issuer %q: err = %v, want an error naming NICE_ISSUER", iss, err)
		}
	}
}

func TestEnvExampleListsEveryVariableAndStarts(t *testing.T) {
	file, err := ReadDotEnv("../../../.env.example")
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{EnvIssuer, EnvListen, EnvJWKSFile, EnvCORSOrigins, EnvServiceAPIKey, "SMOKE_SA_ID", "SMOKE_API_KEY"} {
		if _, ok := file[k]; !ok {
			t.Errorf(".env.example does not list %s", k)
		}
	}
	for _, secret := range []string{EnvServiceAPIKey, "SMOKE_API_KEY"} {
		if file[secret] != "" {
			t.Errorf(".env.example holds a value for the secret %s", secret)
		}
	}
	cfg, err := Load(env(nil), "../../../.env.example")
	if err != nil {
		t.Fatalf(".env.example as .env: %v", err)
	}
	if cfg.Issuer != "http://localhost:8080" || cfg.Listen != DefaultListen {
		t.Fatalf(".env.example gives %+v", cfg)
	}
}

func TestLoadConfigRefusesAMalformedDotEnv(t *testing.T) {
	_, err := Load(env(nil), writeDotEnv(t, "NICE_ISSUER\n"))
	if err == nil || !strings.Contains(err.Error(), "line 1") {
		t.Fatalf("err = %v, want the bad line", err)
	}
}
