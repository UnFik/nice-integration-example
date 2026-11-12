package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"strings"
)

const (
	envIssuer        = "NICE_ISSUER"
	envListen        = "LISTEN_ADDR"
	envJWKSFile      = "JWKS_FILE"
	envCORSOrigins   = "CORS_ALLOWED_ORIGINS"
	envServiceAPIKey = "NICE_SERVICE_API_KEY"
)

const (
	defaultListen      = "127.0.0.1:8081"
	defaultJWKSFile    = "data/jwks.json"
	defaultCORSOrigins = "http://localhost:5180"
)

// Config is what the service needs to run. The slug is not configurable: the
// permission names in the code and in the OpenAPI document depend on it.
type Config struct {
	Issuer      string
	Listen      string
	JWKSFile    string
	CORSOrigins []string
	// ServiceAPIKey is the API key of a platform Service Account with the
	// nice.service role. Empty disables the feed and the online check.
	ServiceAPIKey string
}

type lookupFunc func(key string) (string, bool)

// loadConfig reads the configuration from the environment, falling back to the
// file at dotenv. A variable set in the environment wins over the file, even
// when it is empty. A missing file is not an error.
func loadConfig(lookup lookupFunc, dotenv string) (Config, error) {
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
		Issuer:        get(envIssuer, ""),
		Listen:        get(envListen, defaultListen),
		JWKSFile:      get(envJWKSFile, defaultJWKSFile),
		ServiceAPIKey: get(envServiceAPIKey, ""),
	}
	for o := range strings.SplitSeq(get(envCORSOrigins, defaultCORSOrigins), ",") {
		if o = strings.TrimSpace(o); o != "" {
			cfg.CORSOrigins = append(cfg.CORSOrigins, o)
		}
	}
	if cfg.Issuer == "" {
		return Config{}, fmt.Errorf("config: %s is required (the issuer of NICE, e.g. http://localhost:8080); set it in the environment or in %s", envIssuer, dotenv)
	}
	if u, err := url.Parse(cfg.Issuer); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Config{}, fmt.Errorf("config: %s must be an http or https URL, got %q", envIssuer, cfg.Issuer)
	}
	if cfg.Listen == "" {
		return Config{}, fmt.Errorf("config: %s must not be empty", envListen)
	}
	if cfg.JWKSFile == "" {
		return Config{}, fmt.Errorf("config: %s must not be empty", envJWKSFile)
	}
	return cfg, nil
}

// readDotEnv parses KEY=VALUE lines. It skips blank lines and # comments,
// accepts an "export " prefix, and strips one pair of matching quotes.
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
