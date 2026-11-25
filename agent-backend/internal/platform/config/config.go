package config

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/url"
	"os"
	"strings"
)

const (
	EnvIssuer       = "NICE_ISSUER"
	EnvClientID     = "NICE_CLIENT_ID"
	EnvClientSecret = "NICE_CLIENT_SECRET"
	EnvNadiURL      = "NADI_URL"
	EnvListen       = "LISTEN_ADDR"
	EnvPublicURL    = "PUBLIC_URL"
	EnvJWKSFile     = "JWKS_FILE"
)

const (
	defaultListen    = "127.0.0.1:8083"
	defaultPublicURL = "http://127.0.0.1:8083"
	defaultJWKSFile  = "data/jwks.json"
)

// Config is what the agent needs to run. The slug is not configurable: the permission name in the code and in
// openapi.json depends on it.
type Config struct {
	Issuer string
	// ClientID and ClientSecret are those of the confidential service client bound to agen: the actor of every
	// token exchange. The secret goes to the token endpoint of NICE only.
	ClientID     string
	ClientSecret string
	// NadiURL is called directly with the exchanged token. The SDK refuses http unless the host is a loopback.
	NadiURL string
	Listen  string
	// PublicURL is the address MCP clients use; the resource of the metadata is PublicURL + /mcp.
	PublicURL string
	JWKSFile  string
}

// LogValue keeps the secret out of every log line that prints the config.
func (c Config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("issuer", c.Issuer), slog.String("client_id", c.ClientID),
		slog.String("nadi_url", c.NadiURL),
		slog.String("listen", c.Listen), slog.String("public_url", c.PublicURL))
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
		Issuer:       strings.TrimRight(get(EnvIssuer, ""), "/"),
		ClientID:     get(EnvClientID, ""),
		ClientSecret: get(EnvClientSecret, ""),
		NadiURL:      strings.TrimRight(get(EnvNadiURL, ""), "/"),
		Listen:       get(EnvListen, defaultListen),
		PublicURL:    strings.TrimRight(get(EnvPublicURL, defaultPublicURL), "/"),
		JWKSFile:     get(EnvJWKSFile, defaultJWKSFile),
	}
	required := []struct{ key, value, hint string }{
		{EnvIssuer, cfg.Issuer, "the issuer of NICE, e.g. http://localhost:8080"},
		{EnvClientID, cfg.ClientID, "the client_id of the confidential service client agen-backend"},
		{EnvClientSecret, cfg.ClientSecret, "the secret of the confidential service client agen-backend"},
		{EnvNadiURL, cfg.NadiURL, "the address of nadi, e.g. http://127.0.0.1:8081"},
	}
	for _, r := range required {
		if r.value == "" {
			return Config{}, fmt.Errorf("config: %s is required (%s); set it in the environment or in %s", r.key, r.hint, dotenv)
		}
	}
	for _, u := range []struct{ key, value string }{
		{EnvIssuer, cfg.Issuer}, {EnvNadiURL, cfg.NadiURL}, {EnvPublicURL, cfg.PublicURL},
	} {
		if p, err := url.Parse(u.value); err != nil || (p.Scheme != "http" && p.Scheme != "https") || p.Host == "" {
			return Config{}, fmt.Errorf("config: %s must be an http or https URL, got %q", u.key, u.value)
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
