package auth

import (
	"errors"
	"net/http"

	authrestdto "github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/auth/dto"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/internal/reply"
	sessionusecase "github.com/nice-platform/example/sharedservice-oauth/backend/internal/app/session/usecase"
)

// Logout revokes the refresh token at NICE and removes the session, then
// answers where the browser may end its NICE session too. When NICE does not
// answer the session is kept (503), so the user can try again.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	sess, err := h.Sessions.Session(r.Context(), cookieOf(r))
	if errors.Is(err, sessionusecase.ErrNoSession) {
		h.clearCookie(w)
		reply.JSON(w, http.StatusOK, authrestdto.Logout{EndSessionURL: h.Sessions.EndSessionURL("")})
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
	endSessionURL, err := h.Sessions.Logout(r.Context(), sess)
	if err != nil {
		h.fail(w, err)
		return
	}
	h.clearCookie(w)
	reply.JSON(w, http.StatusOK, authrestdto.Logout{EndSessionURL: endSessionURL})
}
