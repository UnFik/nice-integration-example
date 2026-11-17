// Package auth serves the browser session of the UI: sign-in through NICE,
// the session, its contexts, switching context and signing out. It also puts
// the token of the active context on the item requests that come with the
// session cookie, so they pass the same nicetoken checks as a Bearer caller.
package auth

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	sessionusecase "github.com/nice-platform/example/sharedservice-oauth/backend/internal/app/session/usecase"
)

// CookieName is the session cookie. Cookies of localhost are shared by every
// port, so the name is specific to this example.
const CookieName = "inventaris_session"

type Handler struct {
	Sessions *sessionusecase.Service
	// Perms verifies a token of the active context and returns its perms.
	Perms func(ctx context.Context, token string) ([]string, error)
	// PublicURL is the origin the browser sees; a write with a session cookie
	// must come from it.
	PublicURL string
	Log       *slog.Logger
}

func (h *Handler) secure() bool { return strings.HasPrefix(h.PublicURL, "https://") }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /auth/login", h.Login)
	mux.HandleFunc("GET /auth/callback", h.Callback)
	mux.HandleFunc("GET /auth/session", h.Session)
	mux.HandleFunc("GET /auth/contexts", h.Contexts)
	mux.HandleFunc("POST /auth/context", h.SwitchContext)
	mux.HandleFunc("POST /auth/logout", h.Logout)
}
