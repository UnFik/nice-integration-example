package chat

import (
	"net/http"

	"github.com/nice-platform/example/agent/backend/internal/adapter/rest/chat/dto"
)

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	c, ok := h.caller(w, r)
	if !ok {
		return
	}
	list, err := h.Chat.ListConversations(r.Context(), c)
	if err != nil {
		h.failure(w, err)
		return
	}
	out := dto.ConversationList{Conversations: []dto.Conversation{}, ChatEnabled: h.ChatEnabled}
	for _, conv := range list {
		out.Conversations = append(out.Conversations, dto.FromConversation(conv))
	}
	writeJSON(w, http.StatusOK, out)
}
