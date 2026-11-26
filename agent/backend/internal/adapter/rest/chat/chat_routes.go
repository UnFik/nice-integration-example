// Package chat serves the chat API of the dashboard. Every route runs behind the session middleware, which puts the
// signed-in user (with the token of the active context) in the request; the handlers never see a Bearer token.
package chat

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	calldto "github.com/nice-platform/example/agent/backend/internal/app/call/dto"
	chatusecase "github.com/nice-platform/example/agent/backend/internal/app/chat/usecase"
)

const (
	maxBody = 64 << 10
	// defaultTurnTimeout bounds one turn, which outlives the request that started it.
	defaultTurnTimeout = 5 * time.Minute
)

type Handler struct {
	Chat *chatusecase.Service
	// Caller returns the user the session middleware put in the request.
	Caller      func(*http.Request) (calldto.Caller, bool)
	ChatEnabled bool
	TurnTimeout time.Duration
	Log         *slog.Logger
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /chat/conversations", h.List)
	mux.HandleFunc("POST /chat/conversations", h.Create)
	mux.HandleFunc("GET /chat/conversations/{id}", h.Get)
	mux.HandleFunc("PATCH /chat/conversations/{id}", h.Rename)
	mux.HandleFunc("DELETE /chat/conversations/{id}", h.Delete)
	mux.HandleFunc("POST /chat/conversations/{id}/messages", h.Send)
	mux.HandleFunc("POST /chat/conversations/{id}/approvals/{call_id}", h.Approve)
}

// Routes returns the chat routes as one handler, for the session middleware to wrap.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	h.Register(mux)
	return mux
}

func (h *Handler) caller(w http.ResponseWriter, r *http.Request) (calldto.Caller, bool) {
	c, ok := h.Caller(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "sign in to use the chat")
	}
	return c, ok
}

// turnContext keeps a turn going when the browser leaves: its steps have side effects and are stored as they run.
func (h *Handler) turnContext(r *http.Request) (context.Context, context.CancelFunc) {
	d := h.TurnTimeout
	if d <= 0 {
		d = defaultTurnTimeout
	}
	return context.WithTimeout(context.WithoutCancel(r.Context()), d)
}
