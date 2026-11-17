package auth

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"

	sessionusecase "github.com/nice-platform/example/sharedservice-oauth/backend/internal/app/session/usecase"
)

// Login starts the sign-in: a pending session in the cookie, then NICE.
// prompt=select_account lets the user pick another account.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	sess, authURL, err := h.Sessions.StartLogin(r.Context(), cookieOf(r), r.URL.Query().Get("prompt"))
	if err != nil {
		h.Log.Error("sign-in could not start", slog.String("error", err.Error()))
		h.toUI(w, r, "server_error")
		return
	}
	h.setCookie(w, sess.ID, int(sessionusecase.PendingTTL.Seconds()))
	http.Redirect(w, r, authURL, http.StatusFound)
}

// Callback completes the sign-in and sends the browser to the UI, with
// login_error when it failed. The URL never carries a token.
func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	sess, err := h.Sessions.FinishLogin(r.Context(), cookieOf(r), r.URL.Query())
	var refused *sessionusecase.RefusedError
	var down *sessionusecase.UnavailableError
	switch {
	case err == nil:
		h.setCookie(w, sess.ID, 0)
		h.toUI(w, r, "")
		return
	case errors.Is(err, sessionusecase.ErrStateMismatch):
		h.toUI(w, r, "state_mismatch")
	case errors.As(err, &refused):
		h.toUI(w, r, refused.Code)
	case errors.As(err, &down):
		h.Log.Warn("sign-in: NICE did not answer", slog.String("error", err.Error()))
		h.toUI(w, r, "temporarily_unavailable")
	default:
		h.Log.Error("sign-in failed", slog.String("error", err.Error()))
		h.toUI(w, r, "server_error")
	}
}

// toUI redirects to the UI, with the code of a failed sign-in. A failed
// sign-in leaves no cookie.
func (h *Handler) toUI(w http.ResponseWriter, r *http.Request, loginError string) {
	target := h.PublicURL + "/"
	if loginError != "" {
		h.clearCookie(w)
		target += "?" + url.Values{"login_error": {loginError}}.Encode()
	}
	http.Redirect(w, r, target, http.StatusFound)
}
