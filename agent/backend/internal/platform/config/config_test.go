package config

import (
	"fmt"
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
	if cfg.Listen != "127.0.0.1:8083" || cfg.PublicURL != "http://127.0.0.1:8083" || cfg.JWKSFile != "data/jwks.json" {
		t.Fatalf("defaults = %+v", cfg)
	}
}

func TestLoadConfigReadsDotEnvAndTheEnvironmentWins(t *testing.T) {
	dotenv := writeDotEnv(t, `# comment
NICE_ISSUER="http://localhost:8080/"
export NICE_CLIENT_ID=from-file
NICE_CLIENT_SECRET='secret-from-file'
LISTEN_ADDR=127.0.0.1:9083
`)
	cfg, err := Load(env(map[string]string{EnvClientSecret: "secret-from-env", EnvPublicURL: "http://agent.test/"}), dotenv)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Issuer != "http://localhost:8080" || cfg.ClientID != "from-file" || cfg.ClientSecret != "secret-from-env" ||
		cfg.Listen != "127.0.0.1:9083" || cfg.PublicURL != "http://agent.test" {
		t.Fatalf("config = %+v", cfg)
	}
}

func TestLoadConfigRefusesURLsThatAreNotURLs(t *testing.T) {
	for _, key := range []string{EnvIssuer, EnvPublicURL} {
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
	for _, k := range []string{EnvIssuer, EnvClientID, EnvClientSecret, EnvListen, EnvPublicURL, EnvJWKSFile} {
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
	// Service addresses come from the NICE catalog.
	if _, ok := file["NADI_URL"]; ok {
		t.Errorf(".env.example still lists NADI_URL")
	}
	cfg, err := Load(env(map[string]string{EnvClientID: "id", EnvClientSecret: "s"}), "../../../.env.example")
	if err != nil {
		t.Fatalf(".env.example as .env with the client filled: %v", err)
	}
	if cfg.Listen != defaultListen {
		t.Fatalf(".env.example gives %+v", cfg)
	}
}

func TestLoadConfigRefusesAMalformedDotEnv(t *testing.T) {
	_, err := Load(env(nil), writeDotEnv(t, "NICE_ISSUER\n"))
	if err == nil || !strings.Contains(err.Error(), "line 1") {
		t.Fatalf("err = %v, want the bad line", err)
	}
}

func with(m map[string]string, kv ...string) map[string]string {
	out := map[string]string{}
	for k, v := range m {
		out[k] = v
	}
	for i := 0; i+1 < len(kv); i += 2 {
		out[kv[i]] = kv[i+1]
	}
	return out
}

func TestChatIsOffWithoutAProvider(t *testing.T) {
	cfg, err := Load(env(fullEnv()), "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ChatEnabled() || cfg.ChatMaxSteps != 8 || cfg.SessionIdle.String() != "8h0m0s" {
		t.Fatalf("defaults: %+v", cfg)
	}
	if cfg.RedirectURI != cfg.PublicURL+"/auth/callback" || cfg.PostLogoutURL != cfg.PublicURL+"/" {
		t.Fatalf("dashboard URLs: %q %q", cfg.RedirectURI, cfg.PostLogoutURL)
	}
}

func TestAnthropicProviderDefaultsTheModelAndNeedsAKey(t *testing.T) {
	cfg, err := Load(env(with(fullEnv(), EnvLLMProvider, "Anthropic", EnvLLMAPIKey, "sk-ant-secret")), "")
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.ChatEnabled() || cfg.LLM.Provider != ProviderAnthropic || cfg.LLM.Model != DefaultAnthropicModel {
		t.Fatalf("llm: %v", cfg.LLM)
	}
	if _, err := Load(env(with(fullEnv(), EnvLLMProvider, "anthropic")), ""); err == nil || !strings.Contains(err.Error(), EnvLLMAPIKey) {
		t.Fatalf("anthropic without a key: %v", err)
	}
}

func TestOpenAIProviderNeedsAModelButNoKey(t *testing.T) {
	cfg, err := Load(env(with(fullEnv(), EnvLLMProvider, "openai", EnvLLMModel, "llama3.3", EnvLLMBaseURL, "http://localhost:11434/v1/")), "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LLM.BaseURL != "http://localhost:11434/v1" || cfg.LLM.APIKey != "" {
		t.Fatalf("llm: %v", cfg.LLM)
	}
	if _, err := Load(env(with(fullEnv(), EnvLLMProvider, "openai")), ""); err == nil || !strings.Contains(err.Error(), EnvLLMModel) {
		t.Fatalf("openai without a model: %v", err)
	}
}

func TestUnknownProviderAndBadValuesAreRefused(t *testing.T) {
	for _, kv := range [][]string{
		{EnvLLMProvider, "gemini"},
		{EnvChatSteps, "0"}, {EnvChatSteps, "51"}, {EnvChatSteps, "x"},
		{EnvSessionIdle, "soon"}, {EnvSessionIdle, "-1h"},
		{EnvPostLogout, "not a url"},
		{EnvLLMProvider, "openai", EnvLLMModel, "m", EnvLLMBaseURL, "ftp://x"},
	} {
		if _, err := Load(env(with(fullEnv(), kv...)), ""); err == nil {
			t.Errorf("%v: accepted", kv)
		}
	}
	if _, err := Load(env(with(fullEnv(), EnvPostLogout, "none")), ""); err != nil {
		t.Errorf("POST_LOGOUT_URL=none: %v", err)
	}
}

func TestTheLLMKeyAndDatabaseURLNeverShow(t *testing.T) {
	cfg, err := Load(env(with(fullEnv(), EnvLLMProvider, "anthropic", EnvLLMAPIKey, "sk-ant-topsecret",
		EnvDatabaseURL, "postgres://agen:dbpass@localhost/agen")), "")
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	slog.New(slog.NewJSONHandler(&b, nil)).Info("start", slog.Any("config", cfg))
	for _, s := range []string{fmt.Sprintf("%v %+v", cfg.LLM, cfg.LLM), b.String()} {
		if strings.Contains(s, "topsecret") || strings.Contains(s, "dbpass") {
			t.Fatalf("a secret shows: %s", s)
		}
	}
	if _, err := Load(env(with(fullEnv(), EnvLLMProvider, "x", EnvLLMAPIKey, "sk-ant-topsecret")), ""); err == nil || strings.Contains(err.Error(), "topsecret") {
		t.Fatalf("error: %v", err)
	}
}
