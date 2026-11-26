package chat

import (
	"net/http"

	"github.com/nice-platform/example/agent/backend/internal/adapter/rest/chat/dto"
)

func (h *Handler) Rename(w http.ResponseWriter, r *http.Request) {
	c, ok := h.caller(w, r)
	if !ok {
		return
	}
	var in dto.RenameRequest
	if !decode(w, r, &in) {
		return
	}
	id := r.PathValue("id")
	if err := h.Chat.RenameConversation(r.Context(), c, id, in.Title); err != nil {
		h.failure(w, err)
		return
	}
	conv, _, err := h.Chat.GetConversation(r.Context(), c, id)
	if err != nil {
		h.failure(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]dto.Conversation{"conversation": dto.FromConversation(conv)})
}
