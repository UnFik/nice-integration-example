package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"log/slog"
	"net/http"

	authrestdto "github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/auth/dto"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/internal/reply"
	sessionusecase "github.com/nice-platform/example/sharedservice-oauth/backend/internal/app/session/usecase"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

func (h *Handler) setCookie(w http.ResponseWriter, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: value, Path: "/", MaxAge: maxAge,
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: h.secure()})
}

func (h *Handler) clearCookie(w http.ResponseWriter) { h.setCookie(w, "", -1) }

// keepCookie gives the cookie of a signed-in session the lifetime of the idle
// limit, so the browser keeps it over a restart and drops it when the server
// would end the session. Every use of the session sends it again.
func (h *Handler) keepCookie(w http.ResponseWriter, id string) {
	h.setCookie(w, id, int(h.Sessions.Idle.Seconds()))
}

func cookieOf(r *http.Request) string {
	c, err := r.Cookie(CookieName)
	if err != nil {
		return ""
	}
	return c.Value
}

// sameOrigin is the CSRF check of a write with the session cookie: the header
// only a script of the UI can set, and the Origin of the UI.
func (h *Handler) sameOrigin(r *http.Request, sess domain.Session) bool {
	tok := r.Header.Get("X-CSRF-Token")
	return r.Header.Get("Origin") == h.PublicURL && tok != "" &&
		subtle.ConstantTimeCompare([]byte(tok), []byte(sess.CSRFToken)) == 1
}

func isWrite(method string) bool {
	return method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions
}

// fail answers the errors of the session use cases.
func (h *Handler) fail(w http.ResponseWriter, err error) {
	var refused *sessionusecase.RefusedError
	var down *sessionusecase.UnavailableError
	switch {
	case errors.Is(err, sessionusecase.ErrNoSession):
		h.clearCookie(w)
		reply.Error(w, http.StatusUnauthorized, "unauthenticated", "")
	case errors.Is(err, sessionusecase.ErrSessionEnded):
		h.clearCookie(w)
		reply.Error(w, http.StatusUnauthorized, "session_ended", "")
	case errors.Is(err, sessionusecase.ErrNoContext):
		reply.Error(w, http.StatusConflict, "context_required", "")
	case errors.Is(err, sessionusecase.ErrInvalidContext):
		reply.Error(w, http.StatusBadRequest, "invalid_argument", "")
	case errors.As(err, &refused):
		reply.Error(w, http.StatusConflict, "context_refused", refused.Code)
	case errors.As(err, &down):
		h.Log.Warn("NICE did not answer", slog.String("error", err.Error()))
		reply.Error(w, http.StatusServiceUnavailable, "temporarily_unavailable", "")
	default:
		h.Log.Error("session request failed", slog.String("error", err.Error()))
		reply.Error(w, http.StatusInternalServerError, "server_error", "")
	}
}

// session reads the signed-in session of r, or answers and returns false.
func (h *Handler) session(w http.ResponseWriter, r *http.Request) (domain.Session, bool) {
	sess, err := h.Sessions.Session(r.Context(), cookieOf(r))
	if err != nil {
		h.fail(w, err)
		return domain.Session{}, false
	}
	h.keepCookie(w, sess.ID)
	return sess, true
}

// body is the session as the UI sees it, with the perms of the token of the
// active context. Only a session NICE ended is an error; other failures of the
// token are told in context_error.
func (h *Handler) body(ctx context.Context, sess domain.Session) (authrestdto.Session, error) {
	out := authrestdto.Session{
		User:      authrestdto.User{ID: sess.User.ID, Name: sess.User.Name, Email: sess.User.Email},
		Perms:     []string{},
		CSRFToken: sess.CSRFToken,
	}
	if sess.Context == nil {
		return out, nil
	}
	out.Context = &authrestdto.Context{Org: sess.Context.Org, Project: sess.Context.Project}
	tok, err := h.Sessions.ContextToken(ctx, sess)
	var refused *sessionusecase.RefusedError
	switch {
	case errors.Is(err, sessionusecase.ErrSessionEnded):
		return authrestdto.Session{}, err
	case errors.As(err, &refused):
		out.ContextError = refused.Code
		return out, nil
	case err != nil:
		out.ContextError = "temporarily_unavailable"
		return out, nil
	}
	perms, err := h.Perms(ctx, tok)
	if err != nil {
		out.ContextError = "token_invalid"
		return out, nil
	}
	if perms != nil {
		out.Perms = perms
	}
	return out, nil
}
