package chat

import (
	"net/http"

	"github.com/nice-platform/example/agent/backend/internal/adapter/rest/chat/dto"
)

// Approve gives the user's answer to a call waiting for approval and streams the rest of the turn.
func (h *Handler) Approve(w http.ResponseWriter, r *http.Request) {
	c, ok := h.caller(w, r)
	if !ok {
		return
	}
	var in dto.ApprovalRequest
	if !decode(w, r, &in) {
		return
	}
	if in.Approve == nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "approve must be true or false")
		return
	}
	ctx, cancel := h.turnContext(r)
	defer cancel()
	s := &stream{w: w}
	if err := h.Chat.AnswerApproval(ctx, c, r.PathValue("id"), r.PathValue("call_id"), *in.Approve, s.emit); err != nil && !s.hasStarted() {
		h.failure(w, err)
	}
}
