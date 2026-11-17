package auth

import (
	"net/http"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/internal/reply"
)

// Session answers who is signed in, the active context, its perms and the CSRF token.
func (h *Handler) Session(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.session(w, r)
	if !ok {
		return
	}
	body, err := h.body(r.Context(), sess)
	if err != nil {
		h.fail(w, err)
		return
	}
	reply.JSON(w, http.StatusOK, body)
}
