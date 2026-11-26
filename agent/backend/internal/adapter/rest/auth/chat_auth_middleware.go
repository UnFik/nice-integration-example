package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/ABA-Developer/nice-auth-sdk/verify"

	"github.com/nice-platform/example/agent/backend/internal/adapter/rest/internal/reply"
	calldto "github.com/nice-platform/example/agent/backend/internal/app/call/dto"
	"github.com/nice-platform/example/agent/backend/internal/domain"
)

type callerKey struct{}

// CallerFrom returns the user RequireChatSession put on the request.
func CallerFrom(ctx context.Context) (calldto.Caller, bool) {
	c, ok := ctx.Value(callerKey{}).(calldto.Caller)
	return c, ok
}

// RequireChatSession lets a chat request in with the session cookie only. The
// token of the active context is verified by the verifier of /mcp and must hold
// agen.mcp.use, so the chat can do no more than an MCP client of the same user.
// A request with Authorization is refused: /chat/* is not a Bearer API, and a
// Bearer token there would bypass the CSRF rule.
func (h *Handler) RequireChatSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			reply.Error(w, http.StatusUnauthorized, "unauthenticated", "session_required")
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
		claims, err := h.Verifier.Verify(r.Context(), tok)
		if err != nil {
			var ve *verify.Error
			if errors.As(err, &ve) {
				reply.Error(w, ve.Status, ve.Code, ve.Description)
				return
			}
			h.Log.Error("chat token check failed", slog.String("error", err.Error()))
			reply.Error(w, http.StatusInternalServerError, "server_error", "")
			return
		}
		if !claims.Has(domain.PermUse) {
			reply.Error(w, http.StatusForbidden, "insufficient_scope", "permission_denied")
			return
		}
		c := calldto.Caller{Token: tok, Subject: claims.Subject, OrgID: claims.OrgID, ProjectID: claims.ProjectID}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), callerKey{}, c)))
	})
}
