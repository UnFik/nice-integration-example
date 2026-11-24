package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func env(vars map[string]string) LookupFunc {
	return func(k string) (string, bool) {
		v, ok := vars[k]
		return v, ok
	}
}

// client adds the OAuth client every start needs to vars.
func client(vars map[string]string) map[string]string {
	out := map[string]string{EnvClientID: "cli_test", EnvClientSecret: "s3cret-client"}
	for k, v := range vars {
		out[k] = v
	}
	return out
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
	cfg, err := Load(env(client(map[string]string{EnvIssuer: "http://localhost:8080"})), filepath.Join(t.TempDir(), ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Listen != "127.0.0.1:8081" || cfg.JWKSFile != "data/jwks.json" || cfg.ServiceAPIKey != "" || cfg.DatabaseURL != "" ||
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
	cfg, err := Load(env(client(map[string]string{EnvJWKSFile: "/tmp/from-env.json"})), dotenv)
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
		_, err := Load(env(client(map[string]string{EnvIssuer: iss})), filepath.Join(t.TempDir(), ".env"))
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
	for _, k := range []string{EnvIssuer, EnvListen, EnvJWKSFile, EnvCORSOrigins, EnvServiceAPIKey, EnvDatabaseURL,
		EnvClientID, EnvClientSecret, EnvPublicURL, EnvPostLogoutURL, EnvSessionIdle, EnvWebDir, "SMOKE_SA_ID", "SMOKE_API_KEY"} {
		if _, ok := file[k]; !ok {
			t.Errorf(".env.example does not list %s", k)
		}
	}
	for _, secret := range []string{EnvServiceAPIKey, EnvDatabaseURL, EnvClientSecret, "SMOKE_API_KEY"} {
		if file[secret] != "" {
			t.Errorf(".env.example holds a value for the secret %s", secret)
		}
	}
	cfg, err := Load(env(map[string]string{EnvClientID: "cli_test", EnvClientSecret: "s3cret-client"}), "../../../.env.example")
	if err != nil {
		t.Fatalf(".env.example as .env: %v", err)
	}
	if cfg.Issuer != "http://localhost:8080" || cfg.Listen != DefaultListen {
		t.Fatalf(".env.example gives %+v", cfg)
	}
}

func TestLoadConfigRefusesAMalformedDotEnv(t *testing.T) {
	_, err := Load(env(client(nil)), writeDotEnv(t, "NICE_ISSUER\n"))
	if err == nil || !strings.Contains(err.Error(), "line 1") {
		t.Fatalf("err = %v, want the bad line", err)
	}
}

func TestLoadConfigReadsTheDatabaseURL(t *testing.T) {
	const db = "postgres://nadi:s3cret@localhost:5432/nadi?sslmode=disable"
	cfg, err := Load(env(client(map[string]string{EnvIssuer: "http://localhost:8080", EnvDatabaseURL: db})), filepath.Join(t.TempDir(), ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DatabaseURL != db {
		t.Fatalf("DatabaseURL = %q", cfg.DatabaseURL)
	}
}

func TestLoadConfigRefusesADatabaseURLWithoutPrintingIt(t *testing.T) {
	for _, db := range []string{"mysql://root:s3cret@localhost/nadi", "host=localhost password=s3cret", "postgres://:s3cret@"} {
		_, err := Load(env(client(map[string]string{EnvIssuer: "http://localhost:8080", EnvDatabaseURL: db})), filepath.Join(t.TempDir(), ".env"))
		if err == nil || !strings.Contains(err.Error(), EnvDatabaseURL) {
			t.Errorf("%s: err = %v, want an error naming %s", db, err, EnvDatabaseURL)
			continue
		}
		if strings.Contains(err.Error(), "s3cret") {
			t.Errorf("the error prints the value of %s: %v", EnvDatabaseURL, err)
		}
	}
}

func TestLoadConfigDerivesTheRedirectURI(t *testing.T) {
	cfg, err := Load(env(client(map[string]string{EnvIssuer: "http://localhost:8080", EnvPublicURL: "http://localhost:5181/"})), filepath.Join(t.TempDir(), ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PublicURL != "http://localhost:5181" || cfg.RedirectURI != "http://localhost:5181/auth/callback" {
		t.Fatalf("public %q redirect %q", cfg.PublicURL, cfg.RedirectURI)
	}
	if cfg.SessionIdle != 8*time.Hour || cfg.WebDir != "../web/dist" {
		t.Fatalf("session idle %v web dir %q", cfg.SessionIdle, cfg.WebDir)
	}
}

func TestLoadConfigRefusesToStartWithoutTheClient(t *testing.T) {
	for _, missing := range []string{EnvClientID, EnvClientSecret} {
		vars := client(map[string]string{EnvIssuer: "http://localhost:8080"})
		delete(vars, missing)
		_, err := Load(env(vars), filepath.Join(t.TempDir(), ".env"))
		if err == nil || !strings.Contains(err.Error(), missing) {
			t.Errorf("without %s: err = %v, want an error naming it", missing, err)
			continue
		}
		if strings.Contains(err.Error(), "s3cret-client") {
			t.Errorf("without %s: the error prints the secret: %v", missing, err)
		}
	}
}

func TestLoadConfigRefusesAPublicURLThatIsNotAnOrigin(t *testing.T) {
	for _, pub := range []string{"localhost:8081", "http://localhost:8081/app", "ftp://x", "http://u:p@localhost:8081"} {
		_, err := Load(env(client(map[string]string{EnvIssuer: "http://localhost:8080", EnvPublicURL: pub})), filepath.Join(t.TempDir(), ".env"))
		if err == nil || !strings.Contains(err.Error(), EnvPublicURL) {
			t.Errorf("%s: err = %v, want an error naming %s", pub, err, EnvPublicURL)
		}
	}
	_, err := Load(env(client(map[string]string{EnvIssuer: "http://localhost:8080", EnvSessionIdle: "forever"})), filepath.Join(t.TempDir(), ".env"))
	if err == nil || !strings.Contains(err.Error(), EnvSessionIdle) {
		t.Errorf("err = %v, want an error naming %s", err, EnvSessionIdle)
	}
}

func TestLoadConfigPostLogoutURL(t *testing.T) {
	load := func(vars map[string]string) (Config, error) {
		vars[EnvIssuer] = "http://localhost:8080"
		vars[EnvPublicURL] = "http://localhost:5181"
		return Load(env(client(vars)), filepath.Join(t.TempDir(), ".env"))
	}
	if cfg, err := load(map[string]string{}); err != nil || cfg.PostLogoutURL != "http://localhost:5181/" {
		t.Fatalf("default = %q, %v; want PUBLIC_URL + /", cfg.PostLogoutURL, err)
	}
	if cfg, err := load(map[string]string{EnvPostLogoutURL: "none"}); err != nil || cfg.PostLogoutURL != "" {
		t.Fatalf("none = %q, %v; want the page of NICE", cfg.PostLogoutURL, err)
	}
	if cfg, err := load(map[string]string{EnvPostLogoutURL: "https://nadi.example.com/signed-out"}); err != nil ||
		cfg.PostLogoutURL != "https://nadi.example.com/signed-out" {
		t.Fatalf("set = %q, %v", cfg.PostLogoutURL, err)
	}
	if _, err := load(map[string]string{EnvPostLogoutURL: "/signed-out"}); err == nil || !strings.Contains(err.Error(), EnvPostLogoutURL) {
		t.Fatalf("a path is refused: %v", err)
	}
}
