package chat

import "net/http"

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	c, ok := h.caller(w, r)
	if !ok {
		return
	}
	if err := h.Chat.DeleteConversation(r.Context(), c, r.PathValue("id")); err != nil {
		h.failure(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
