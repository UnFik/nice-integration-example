package auth

import (
	"net/http"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/internal/reply"
)

// WithSession puts the token of the active context on a device request that
// comes with the session cookie and no Authorization, then hands it to next:
// the nicetoken chain verifies it as any Bearer token, so permissions,
// tenant, feed and sensitive checks hold for both paths. A request with
// Authorization is a Bearer caller and the cookie is ignored.
func (h *Handler) WithSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" || cookieOf(r) == "" {
			next.ServeHTTP(w, r)
			return
		}
		sess, ok := h.session(w, r)
		if !ok {
			return
		}
		if isWrite(r.Method) && !h.sameOrigin(r, sess) {
			reply.Error(w, http.StatusForbidden, "csrf", "")
			return
		}
		tok, err := h.Sessions.ContextToken(r.Context(), sess)
		if err != nil {
			h.fail(w, err)
			return
		}
		r = r.Clone(r.Context())
		r.Header.Set("Authorization", "Bearer "+tok)
		next.ServeHTTP(w, r)
	})
}
