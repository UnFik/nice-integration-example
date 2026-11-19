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
	"time"
)

const (
	EnvIssuer        = "NICE_ISSUER"
	EnvAccountID     = "NICE_SA_ID"
	EnvAPIKey        = "NICE_SA_API_KEY"
	EnvNadiURL       = "NADI_URL"
	EnvWatchInterval = "WATCH_INTERVAL"
)

const (
	defaultNadiURL       = "http://127.0.0.1:8081"
	defaultWatchInterval = 30 * time.Second
)

// Config is what the client needs to run. The slug of the service is not configurable: the token is for nadi.
type Config struct {
	Issuer string
	// AccountID is the id of the Service Account (sa_...), sent as client_id.
	AccountID string
	// APIKey is the API key of the Service Account, sent as client_secret to the token endpoint of NICE only.
	APIKey string
	// NadiURL is called directly with the token NICE issued.
	NadiURL       string
	WatchInterval time.Duration
}

// LogValue keeps the API key out of every log line that prints the config.
func (c Config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("issuer", c.Issuer), slog.String("service_account", c.AccountID),
		slog.String("nadi_url", c.NadiURL), slog.Duration("watch_interval", c.WatchInterval))
}

type LookupFunc func(key string) (string, bool)

// Load reads the configuration from the environment, falling back to the file at dotenv. A variable set in
// the environment wins over the file, even when it is empty. A missing file is not an error. No error names the
// value of a secret.
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
		Issuer:    strings.TrimRight(get(EnvIssuer, ""), "/"),
		AccountID: get(EnvAccountID, ""),
		APIKey:    get(EnvAPIKey, ""),
		NadiURL:   strings.TrimRight(get(EnvNadiURL, defaultNadiURL), "/"),
	}
	required := []struct{ key, value, hint string }{
		{EnvIssuer, cfg.Issuer, "the issuer of NICE, e.g. http://localhost:8080"},
		{EnvAccountID, cfg.AccountID, "the id of the Service Account, sa_..."},
		{EnvAPIKey, cfg.APIKey, "the API key of the Service Account"},
	}
	for _, r := range required {
		if r.value == "" {
			return Config{}, fmt.Errorf("config: %s is required (%s); set it in the environment or in %s", r.key, r.hint, dotenv)
		}
	}
	for _, u := range []struct{ key, value string }{{EnvIssuer, cfg.Issuer}, {EnvNadiURL, cfg.NadiURL}} {
		if p, err := url.Parse(u.value); err != nil || (p.Scheme != "http" && p.Scheme != "https") || p.Host == "" {
			return Config{}, fmt.Errorf("config: %s must be an http or https URL, got %q", u.key, u.value)
		}
	}
	cfg.WatchInterval = defaultWatchInterval
	if raw := get(EnvWatchInterval, ""); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil || d <= 0 {
			return Config{}, fmt.Errorf("config: %s must be a positive duration such as 30s, got %q", EnvWatchInterval, raw)
		}
		cfg.WatchInterval = d
	}
	return cfg, nil
}

// readDotEnv parses KEY=VALUE lines. It skips blank lines and # comments, accepts an "export " prefix, and strips
// one pair of matching quotes. An error names the line, never its content: the line may hold the API key.
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
