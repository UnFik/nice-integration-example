// Package config reads the service configuration from the environment and the .env file.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	EnvIssuer        = "NICE_ISSUER"
	EnvListen        = "LISTEN_ADDR"
	EnvJWKSFile      = "JWKS_FILE"
	EnvCORSOrigins   = "CORS_ALLOWED_ORIGINS"
	EnvDatabaseURL   = "DATABASE_URL"
	EnvClientID      = "NICE_CLIENT_ID"
	EnvClientSecret  = "NICE_CLIENT_SECRET"
	EnvPublicURL     = "PUBLIC_URL"
	EnvPostLogoutURL = "POST_LOGOUT_URL"
	EnvSessionIdle   = "SESSION_IDLE_TIMEOUT"
	EnvWebDir        = "WEB_DIR"
)

const (
	DefaultListen      = "127.0.0.1:8081"
	defaultJWKSFile    = "data/jwks.json"
	defaultCORSOrigins = "http://localhost:5180"
	defaultPublicURL   = "http://localhost:8081"
	defaultSessionIdle = "8h"
	defaultWebDir      = "../web/dist"
	// CallbackPath is where NICE sends the browser back after the sign-in.
	CallbackPath = "/auth/callback"
)

// Config is what the service needs to run. The slug is not configurable: the
// permission names in the code and in the OpenAPI document depend on it.
type Config struct {
	Issuer      string
	Listen      string
	JWKSFile    string
	CORSOrigins []string
	// DatabaseURL is the PostgreSQL URL of the device store. Empty keeps the
	// devices in memory. It may hold a password, so it is never printed.
	DatabaseURL string
	// ClientID and ClientSecret are those of the confidential OAuth client bound
	// to nadi, used by the backend to sign users in. The secret is sent to
	// NICE only and never printed.
	ClientID     string
	ClientSecret string
	// PublicURL is the origin the browser sees (scheme, host and port, no path):
	// the Vite dev server in development, the backend itself in a build.
	PublicURL string
	// RedirectURI is PublicURL + CallbackPath; it must be registered on the client.
	RedirectURI string
	// PostLogoutURL is where NICE sends the browser after "sign out" ended its
	// session; it must be registered as a post-logout redirect URI of the client.
	// Default PublicURL + "/"; "none" leaves the browser on the page of NICE.
	PostLogoutURL string
	// SessionIdle ends a browser session that was not used for this long.
	SessionIdle time.Duration
	// WebDir holds the built UI (web/dist); a missing directory serves no UI.
	WebDir string
}

type LookupFunc func(key string) (string, bool)

// Load reads the configuration from the environment, falling back to the
// file at dotenv. A variable set in the environment wins over the file, even
// when it is empty. A missing file is not an error.
func Load(lookup LookupFunc, dotenv string) (Config, error) {
	file, err := ReadDotEnv(dotenv)
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
		Issuer:       get(EnvIssuer, ""),
		Listen:       get(EnvListen, DefaultListen),
		JWKSFile:     get(EnvJWKSFile, defaultJWKSFile),
		DatabaseURL:  get(EnvDatabaseURL, ""),
		ClientID:     get(EnvClientID, ""),
		ClientSecret: get(EnvClientSecret, ""),
		PublicURL:    strings.TrimRight(get(EnvPublicURL, defaultPublicURL), "/"),
		WebDir:       get(EnvWebDir, defaultWebDir),
	}
	for o := range strings.SplitSeq(get(EnvCORSOrigins, defaultCORSOrigins), ",") {
		if o = strings.TrimSpace(o); o != "" {
			cfg.CORSOrigins = append(cfg.CORSOrigins, o)
		}
	}
	if cfg.Issuer == "" {
		return Config{}, fmt.Errorf("config: %s is required (the issuer of NICE, e.g. http://localhost:8080); set it in the environment or in %s", EnvIssuer, dotenv)
	}
	if u, err := url.Parse(cfg.Issuer); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Config{}, fmt.Errorf("config: %s must be an http or https URL, got %q", EnvIssuer, cfg.Issuer)
	}
	if cfg.Listen == "" {
		return Config{}, fmt.Errorf("config: %s must not be empty", EnvListen)
	}
	if cfg.JWKSFile == "" {
		return Config{}, fmt.Errorf("config: %s must not be empty", EnvJWKSFile)
	}
	if cfg.DatabaseURL != "" {
		// The value is left out of the message: it may hold a password.
		if u, err := url.Parse(cfg.DatabaseURL); err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Host == "" {
			return Config{}, fmt.Errorf("config: %s must be a postgres:// URL (e.g. postgres://user:password@localhost:5432/nadi), or empty to keep devices in memory", EnvDatabaseURL)
		}
	}
	if cfg.ClientID == "" {
		return Config{}, fmt.Errorf("config: %s is required (the client_id of the confidential OAuth client bound to nadi); set it in the environment or in %s", EnvClientID, dotenv)
	}
	if cfg.ClientSecret == "" {
		return Config{}, fmt.Errorf("config: %s is required (the secret of the OAuth client %s); set it in the environment or in %s", EnvClientSecret, EnvClientID, dotenv)
	}
	if u, err := url.Parse(cfg.PublicURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
		u.Path != "" || u.RawQuery != "" || u.User != nil {
		return Config{}, fmt.Errorf("config: %s must be an origin such as http://localhost:8081 (scheme, host and port only), got %q", EnvPublicURL, cfg.PublicURL)
	}
	cfg.RedirectURI = cfg.PublicURL + CallbackPath
	switch post := get(EnvPostLogoutURL, ""); post {
	case "":
		cfg.PostLogoutURL = cfg.PublicURL + "/"
	case "none":
	default:
		if u, err := url.Parse(post); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return Config{}, fmt.Errorf("config: %s must be an http or https URL, or none, got %q", EnvPostLogoutURL, post)
		}
		cfg.PostLogoutURL = post
	}
	idle := get(EnvSessionIdle, defaultSessionIdle)
	d, err := time.ParseDuration(idle)
	if err != nil || d <= 0 {
		return Config{}, fmt.Errorf("config: %s must be a positive duration such as 8h, got %q", EnvSessionIdle, idle)
	}
	cfg.SessionIdle = d
	return cfg, nil
}

// ReadDotEnv parses KEY=VALUE lines. It skips blank lines and # comments,
// accepts an "export " prefix, and strips one pair of matching quotes.
func ReadDotEnv(path string) (map[string]string, error) {
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
