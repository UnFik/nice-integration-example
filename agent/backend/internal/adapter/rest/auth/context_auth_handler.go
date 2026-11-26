package auth

import (
	"encoding/json"
	"net/http"

	authrestdto "github.com/nice-platform/example/agent/backend/internal/adapter/rest/auth/dto"
	"github.com/nice-platform/example/agent/backend/internal/adapter/rest/internal/reply"
	"github.com/nice-platform/example/agent/backend/internal/domain"
)

// SwitchContext moves the session to another organization and project with
// one refresh. A refusal (409 context_refused) keeps the active context.
func (h *Handler) SwitchContext(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.session(w, r)
	if !ok {
		return
	}
	if !h.sameOrigin(r, sess) {
		reply.Error(w, http.StatusForbidden, "csrf", "")
		return
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10))
	dec.DisallowUnknownFields()
	var in authrestdto.Context
	if err := dec.Decode(&in); err != nil || in.Org == "" || in.Project == "" {
		reply.Error(w, http.StatusBadRequest, "invalid_argument", "")
		return
	}
	next, err := h.Sessions.SwitchContext(r.Context(), sess, domain.Context{Org: in.Org, Project: in.Project})
	if err != nil {
		h.fail(w, err)
		return
	}
	body, err := h.body(r.Context(), next)
	if err != nil {
		h.fail(w, err)
		return
	}
	reply.JSON(w, http.StatusOK, body)
}
