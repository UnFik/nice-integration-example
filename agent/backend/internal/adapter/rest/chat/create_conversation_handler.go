package chat

import (
	"net/http"

	"github.com/nice-platform/example/agent/backend/internal/adapter/rest/chat/dto"
)

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	c, ok := h.caller(w, r)
	if !ok {
		return
	}
	var in dto.CreateRequest
	if !decode(w, r, &in) {
		return
	}
	conv, err := h.Chat.CreateConversation(r.Context(), c, in.Title)
	if err != nil {
		h.failure(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]dto.Conversation{"conversation": dto.FromConversation(conv)})
}
