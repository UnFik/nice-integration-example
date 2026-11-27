package config

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	EnvIssuer       = "NICE_ISSUER"
	EnvClientID     = "NICE_CLIENT_ID"
	EnvClientSecret = "NICE_CLIENT_SECRET"
	EnvListen       = "LISTEN_ADDR"
	EnvPublicURL    = "PUBLIC_URL"
	EnvJWKSFile     = "JWKS_FILE"
	EnvPostLogout   = "POST_LOGOUT_URL"
	EnvSessionIdle  = "SESSION_IDLE_TIMEOUT"
	EnvWebDir       = "WEB_DIR"
	EnvDatabaseURL  = "DATABASE_URL"
	EnvLLMProvider  = "LLM_PROVIDER"
	EnvLLMModel     = "LLM_MODEL"
	EnvLLMBaseURL   = "LLM_BASE_URL"
	EnvLLMAPIKey    = "LLM_API_KEY"
	EnvChatSteps    = "CHAT_MAX_STEPS"
)

// LLM providers. An empty provider turns the chat off; /mcp works without it.
const (
	ProviderAnthropic = "anthropic"
	ProviderOpenAI    = "openai"
	// DefaultAnthropicModel is the model of the anthropic provider when LLM_MODEL is empty.
	DefaultAnthropicModel = "claude-sonnet-5-5"
	// CallbackPath is where NICE sends the browser back after the dashboard sign-in.
	CallbackPath = "/auth/callback"
	maxChatSteps = 50
)

const (
	defaultListen    = "127.0.0.1:8083"
	defaultPublicURL = "http://127.0.0.1:8083"
	defaultJWKSFile  = "data/jwks.json"
	defaultIdle      = "360h"
	defaultWebDir    = "../web/dist"
	defaultChatSteps = "8"
)

// Config is what the agent needs to run. The slug is not configurable: the permission name in the code and in
// openapi.json depends on it.
type Config struct {
	Issuer string
	// ClientID and ClientSecret are those of the confidential service client bound to agen: the actor of every
	// token exchange. The secret goes to the token endpoint of NICE only.
	ClientID     string
	ClientSecret string
	Listen       string
	// PublicURL is the address MCP clients use; the resource of the metadata is PublicURL + /mcp.
	PublicURL string
	JWKSFile  string
	// RedirectURI is PublicURL + CallbackPath: the dashboard sign-in callback, registered on the client.
	RedirectURI string
	// PostLogoutURL is where NICE sends the browser after the dashboard sign-out ended its session. Default
	// PublicURL + "/"; "none" leaves the browser on the page of NICE.
	PostLogoutURL string
	// SessionIdle ends a dashboard session that was not used for this long.
	SessionIdle time.Duration
	// WebDir holds the built dashboard (web/dist); a missing directory serves no UI.
	WebDir string
	// DatabaseURL is the PostgreSQL URL of the conversation history. Empty keeps it in memory. It may hold a
	// password, so it is never printed.
	DatabaseURL string
	// LLM is the model of the chat. An empty Provider turns the chat off.
	LLM LLM
	// ChatMaxSteps bounds the model calls of one chat turn.
	ChatMaxSteps int
}

// LLM is the provider of the chat model. APIKey is a secret: it goes to the provider only.
type LLM struct {
	Provider string
	Model    string
	BaseURL  string
	APIKey   string
}

// String keeps the key out of %v and %+v.
func (l LLM) String() string {
	return fmt.Sprintf("{provider:%s model:%s base_url:%s api_key:%s}", l.Provider, l.Model, l.BaseURL, redacted(l.APIKey))
}

func redacted(s string) string {
	if s == "" {
		return ""
	}
	return "[redacted]"
}

// ChatEnabled reports whether a chat model is configured.
func (c Config) ChatEnabled() bool { return c.LLM.Provider != "" }

// LogValue keeps the secret out of every log line that prints the config.
func (c Config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("issuer", c.Issuer), slog.String("client_id", c.ClientID),
		slog.String("listen", c.Listen), slog.String("public_url", c.PublicURL),
		slog.Bool("database", c.DatabaseURL != ""), slog.String("llm_provider", c.LLM.Provider),
		slog.String("llm_model", c.LLM.Model), slog.String("llm_base_url", c.LLM.BaseURL))
}

type LookupFunc func(key string) (string, bool)

// Load reads the configuration from the environment, falling back to the file at dotenv. A variable set in
// the environment wins over the file, even when it is empty. A missing file is not an error.
func Load(lookup LookupFunc, dotenv string) (Config, error) {
	file, err := readDotEnv(dotenv)
	if err != nil {
		return Config{}, err
	}
	get := func(key, def string) string {
		if v, ok := lookup(key); ok {
			return strings.TrimSpace(v)
		}
		if v, ok := file[key]; ok {
			return v
		}
		return def
	}
	cfg := Config{
		Issuer:        strings.TrimRight(get(EnvIssuer, ""), "/"),
		ClientID:      get(EnvClientID, ""),
		ClientSecret:  get(EnvClientSecret, ""),
		Listen:        get(EnvListen, defaultListen),
		PublicURL:     strings.TrimRight(get(EnvPublicURL, defaultPublicURL), "/"),
		JWKSFile:      get(EnvJWKSFile, defaultJWKSFile),
		PostLogoutURL: get(EnvPostLogout, ""),
		WebDir:        get(EnvWebDir, defaultWebDir),
		DatabaseURL:   get(EnvDatabaseURL, ""),
		LLM: LLM{
			Provider: strings.ToLower(get(EnvLLMProvider, "")),
			Model:    get(EnvLLMModel, ""),
			BaseURL:  strings.TrimRight(get(EnvLLMBaseURL, ""), "/"),
			APIKey:   get(EnvLLMAPIKey, ""),
		},
	}
	cfg.RedirectURI = cfg.PublicURL + CallbackPath
	if cfg.PostLogoutURL == "" {
		cfg.PostLogoutURL = cfg.PublicURL + "/"
	}
	idle, err := time.ParseDuration(get(EnvSessionIdle, defaultIdle))
	if err != nil || idle <= 0 {
		return Config{}, fmt.Errorf("config: %s must be a positive Go duration such as 8h", EnvSessionIdle)
	}
	cfg.SessionIdle = idle
	steps, err := strconv.Atoi(get(EnvChatSteps, defaultChatSteps))
	if err != nil || steps < 1 || steps > maxChatSteps {
		return Config{}, fmt.Errorf("config: %s must be a whole number from 1 to %d", EnvChatSteps, maxChatSteps)
	}
	cfg.ChatMaxSteps = steps
	if err := checkLLM(&cfg.LLM); err != nil {
		return Config{}, err
	}
	required := []struct{ key, value, hint string }{
		{EnvIssuer, cfg.Issuer, "the issuer of NICE, e.g. http://localhost:8080"},
		{EnvClientID, cfg.ClientID, "the client_id of the confidential service client agen-backend"},
		{EnvClientSecret, cfg.ClientSecret, "the secret of the confidential service client agen-backend"},
	}
	for _, r := range required {
		if r.value == "" {
			return Config{}, fmt.Errorf("config: %s is required (%s); set it in the environment or in %s", r.key, r.hint, dotenv)
		}
	}
	for _, u := range []struct{ key, value string }{
		{EnvIssuer, cfg.Issuer}, {EnvPublicURL, cfg.PublicURL},
	} {
		if p, err := url.Parse(u.value); err != nil || (p.Scheme != "http" && p.Scheme != "https") || p.Host == "" {
			return Config{}, fmt.Errorf("config: %s must be an http or https URL, got %q", u.key, u.value)
		}
	}
	if cfg.PostLogoutURL != "none" {
		if p, err := url.Parse(cfg.PostLogoutURL); err != nil || (p.Scheme != "http" && p.Scheme != "https") || p.Host == "" {
			return Config{}, fmt.Errorf("config: %s must be an http or https URL or none, got %q", EnvPostLogout, cfg.PostLogoutURL)
		}
	}
	if cfg.Listen == "" {
		return Config{}, fmt.Errorf("config: %s must not be empty", EnvListen)
	}
	if cfg.JWKSFile == "" {
		return Config{}, fmt.Errorf("config: %s must not be empty", EnvJWKSFile)
	}
	return cfg, nil
}

// checkLLM fills the default model and refuses a provider the agent does not know. The key is never in an error.
func checkLLM(l *LLM) error {
	switch l.Provider {
	case "":
		return nil
	case ProviderAnthropic:
		if l.Model == "" {
			l.Model = DefaultAnthropicModel
		}
		if l.APIKey == "" {
			return fmt.Errorf("config: %s is required for %s=%s", EnvLLMAPIKey, EnvLLMProvider, ProviderAnthropic)
		}
	case ProviderOpenAI:
		// The key is optional: a local OpenAI-compatible server such as Ollama needs none.
		if l.Model == "" {
			return fmt.Errorf("config: %s is required for %s=%s", EnvLLMModel, EnvLLMProvider, ProviderOpenAI)
		}
	default:
		return fmt.Errorf("config: %s must be %s, %s, or empty, got %q", EnvLLMProvider, ProviderAnthropic, ProviderOpenAI, l.Provider)
	}
	if l.BaseURL != "" {
		if p, err := url.Parse(l.BaseURL); err != nil || (p.Scheme != "http" && p.Scheme != "https") || p.Host == "" {
			return fmt.Errorf("config: %s must be an http or https URL, got %q", EnvLLMBaseURL, l.BaseURL)
		}
	}
	return nil
}

// readDotEnv parses KEY=VALUE lines. It skips blank lines and # comments, accepts an "export " prefix, and strips
// one pair of matching quotes.
func readDotEnv(path string) (map[string]string, error) {
	out := map[string]string{}
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return nil, fmt.Errorf("config: %s line %d is not KEY=VALUE", path, n)
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
			value = value[1 : len(value)-1]
		}
		out[key] = value
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	return out, nil
}
