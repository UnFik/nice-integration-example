package auth

import (
	"errors"
	"net/http"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/internal/reply"
	sessionusecase "github.com/nice-platform/example/sharedservice-oauth/backend/internal/app/session/usecase"
)

// Logout revokes the refresh token at NICE and removes the session. When NICE
// does not answer the session is kept (503), so the user can try again.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	sess, err := h.Sessions.Session(r.Context(), cookieOf(r))
	if errors.Is(err, sessionusecase.ErrNoSession) {
		h.clearCookie(w)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		h.fail(w, err)
		return
	}
	if !h.sameOrigin(r, sess) {
		reply.Error(w, http.StatusForbidden, "csrf", "")
		return
	}
	if err := h.Sessions.Logout(r.Context(), sess); err != nil {
		h.fail(w, err)
		return
	}
	h.clearCookie(w)
	w.WriteHeader(http.StatusNoContent)
}
