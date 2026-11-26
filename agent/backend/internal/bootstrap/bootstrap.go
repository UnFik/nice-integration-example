// Package bootstrap assembles the agent: configuration, token verification, the exchange client, the call use
// case, the dashboard session, and the HTTP server. It is the only place that knows every layer.
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/ABA-Developer/nice-auth-sdk/credentials"
	"github.com/ABA-Developer/nice-auth-sdk/verify"

	mcpadapter "github.com/nice-platform/example/agent/backend/internal/adapter/mcp"
	"github.com/nice-platform/example/agent/backend/internal/adapter/niceexchange"
	"github.com/nice-platform/example/agent/backend/internal/adapter/rest/auth"
	"github.com/nice-platform/example/agent/backend/internal/adapter/rest/web"
	callusecase "github.com/nice-platform/example/agent/backend/internal/app/call/usecase"
	"github.com/nice-platform/example/agent/backend/internal/domain"
	"github.com/nice-platform/example/agent/backend/internal/platform/config"
)

// Run loads the configuration and serves until ctx ends.
func Run(ctx context.Context, lookup config.LookupFunc, dotenv string, log *slog.Logger) error {
	cfg, err := config.Load(lookup, dotenv)
	if err != nil {
		return err
	}
	// niceauth/verify writes the JWKS copy through a temporary file in the same directory.
	if err := os.MkdirAll(filepath.Dir(cfg.JWKSFile), 0o700); err != nil {
		return fmt.Errorf("JWKS directory: %w", err)
	}
	v, err := verify.New(VerifierConfig(cfg, log))
	if err != nil {
		return err
	}
	go v.Run(ctx)
	// One client for NICE and the services: the exchange and the tool call share its connection pool.
	calls, err := NewCallService(cfg, &http.Client{Timeout: 30 * time.Second}, log)
	if err != nil {
		return err
	}

	a, err := NewAuth(cfg, v, log)
	if err != nil {
		return err
	}
	a.Start(ctx)

	store, closeStore, err := NewConversations(ctx, cfg)
	if err != nil {
		return fmt.Errorf("conversation store: %w", err)
	}
	defer closeStore()
	// The model has its own client: a turn's model call can take far longer than a call to NICE.
	chat := NewChatHandler(cfg, NewChatService(cfg, store, calls, &http.Client{Timeout: 5 * time.Minute}, log), log)

	handler := NewHandler(Deps{Verifier: v, Calls: calls, Config: cfg, Auth: a.Handler, Chat: chat})
	srv := &http.Server{Addr: cfg.Listen, Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	log.Info("agen listening", slog.Any("config", cfg), slog.String("mcp", cfg.PublicURL+mcpadapter.MCPPath),
		slog.Bool("ui", hasUI(cfg.WebDir)), slog.Bool("chat", cfg.ChatEnabled()))
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// VerifierConfig wires niceauth/verify for the agent. Tests reuse it and only swap the key source.
func VerifierConfig(cfg config.Config, log *slog.Logger) verify.Config {
	return verify.Config{
		Issuer: cfg.Issuer, Service: domain.ServiceSlug, JWKSFile: cfg.JWKSFile, Log: log,
		// A caller with more permissions than a token carries gets perms_ref in place of perms. The resolver asks NICE
		// once for the list it names with the token of the request itself, so it needs no credential of NICE.
		Resolver: verify.NewHTTPResolver(cfg.Issuer),
	}
}

// NewCallService wires the call use case to the exchange client of the agent's own service client: the service map,
// the operations, and the calls all go through it. No service address is configured: each comes from the catalog.
func NewCallService(cfg config.Config, hc *http.Client, log *slog.Logger) (*callusecase.Service, error) {
	x, err := niceexchange.New(credentials.ExchangeConfig{
		Issuer: cfg.Issuer, ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret, HTTP: hc, Log: log,
	})
	if err != nil {
		return nil, err
	}
	return &callusecase.Service{
		Discoverer: x,
		Operator:   x,
		ClientVars: config.EnvClientID + " and " + config.EnvClientSecret,
		Log:        log,
	}, nil
}

// Deps is what NewHandler assembles. Auth and Chat are optional.
type Deps struct {
	Verifier *verify.Verifier
	Calls    *callusecase.Service
	Config   config.Config
	// Auth serves /auth/* and guards /chat/; nil serves /mcp and the public endpoints only.
	Auth *auth.Handler
	// Chat serves /chat/ behind Auth.RequireChatSession; nil answers 404 there.
	Chat http.Handler
}

// NewHandler assembles the HTTP routes of the agent on one mux. /mcp and the public endpoints are those of the MCP
// adapter, unchanged: /mcp takes a Bearer token only and ignores the session cookie. /auth/* and /chat/ take the
// session cookie only. The built UI, when WEB_DIR holds one, answers every other GET.
func NewHandler(d Deps) http.Handler {
	mcpHandler := mcpadapter.NewHandler(mcpadapter.Deps{Verifier: d.Verifier, Calls: d.Calls, Issuer: d.Config.Issuer,
		PublicURL: d.Config.PublicURL})
	mux := http.NewServeMux()
	for _, p := range []string{mcpadapter.MCPPath, "/healthz", "/openapi.json", "/.well-known/"} {
		mux.Handle(p, mcpHandler)
	}
	if d.Auth != nil {
		d.Auth.Register(mux)
		chat := d.Chat
		if chat == nil {
			chat = http.NotFoundHandler()
		}
		mux.Handle(ChatPrefix, d.Auth.RequireChatSession(chat))
	}
	web.Register(mux, d.Config.WebDir)
	return mux
}

// ChatPrefix is where the chat API is mounted.
const ChatPrefix = "/chat/"

func hasUI(dir string) bool {
	st, err := os.Stat(filepath.Join(dir, "index.html"))
	return err == nil && !st.IsDir()
}
