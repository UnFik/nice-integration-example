package chat

import (
	"net/http"

	"github.com/nice-platform/example/agent/backend/internal/adapter/rest/chat/dto"
)

// Send runs one turn and streams its steps as server-sent events.
func (h *Handler) Send(w http.ResponseWriter, r *http.Request) {
	c, ok := h.caller(w, r)
	if !ok {
		return
	}
	var in dto.SendRequest
	if !decode(w, r, &in) {
		return
	}
	ctx, cancel := h.turnContext(r)
	defer cancel()
	s := &stream{w: w}
	if err := h.Chat.RunTurn(ctx, c, r.PathValue("id"), in.Text, s.emit); err != nil && !s.hasStarted() {
		h.failure(w, err)
	}
}
