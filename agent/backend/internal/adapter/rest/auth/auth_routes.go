// Package auth serves the browser session of the dashboard: sign-in through NICE,
// the session, its contexts, switching context and signing out. It also guards
// the chat routes: they take the session cookie only, never a Bearer token.
package auth

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/ABA-Developer/nice-auth-sdk/verify"

	sessionusecase "github.com/nice-platform/example/agent/backend/internal/app/session/usecase"
)

// CookieName is the session cookie. Cookies of localhost are shared by every
// port, so the name is specific to this example.
const CookieName = "agen_session"

type Handler struct {
	Sessions *sessionusecase.Service
	// Verifier is the one of /mcp: a token of the active context passes the
	// same checks as a Bearer caller.
	Verifier *verify.Verifier
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
