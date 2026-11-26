package chat

import (
	"net/http"

	"github.com/nice-platform/example/agent/backend/internal/adapter/rest/chat/dto"
)

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	c, ok := h.caller(w, r)
	if !ok {
		return
	}
	conv, msgs, err := h.Chat.GetConversation(r.Context(), c, r.PathValue("id"))
	if err != nil {
		h.failure(w, err)
		return
	}
	out := dto.ConversationDetail{Conversation: dto.FromConversation(conv), Messages: []dto.Message{}}
	for _, m := range msgs {
		out.Messages = append(out.Messages, dto.FromMessage(m))
	}
	writeJSON(w, http.StatusOK, out)
}
