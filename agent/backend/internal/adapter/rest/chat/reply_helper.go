package chat

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/nice-platform/example/agent/backend/internal/adapter/rest/chat/dto"
	"github.com/nice-platform/example/agent/backend/internal/domain"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, dto.Error{Error: code, Message: message})
}

// failure answers an error of the chat use case. Unknown errors are logged and answered without detail.
func (h *Handler) failure(w http.ResponseWriter, err error) {
	var invalid domain.InvalidInputError
	var long domain.TooLongError
	switch {
	case errors.Is(err, domain.ErrConversationNotFound):
		writeError(w, http.StatusNotFound, "not_found", "conversation not found")
	case errors.Is(err, domain.ErrCallNotPending):
		writeError(w, http.StatusConflict, "call_not_pending", "this call does not wait for an approval")
	case errors.Is(err, domain.ErrTurnInProgress):
		writeError(w, http.StatusConflict, "turn_in_progress", "a turn of this conversation is running; wait for it to end")
	case errors.Is(err, domain.ErrChatOff):
		writeError(w, http.StatusServiceUnavailable, "chat_not_configured", "the chat is not configured on this agent (LLM_PROVIDER is empty)")
	case errors.As(err, &long):
		writeError(w, http.StatusRequestEntityTooLarge, "message_too_long", "the message is longer than 8000 characters")
	case errors.As(err, &invalid):
		writeError(w, http.StatusBadRequest, "invalid_input", invalid.Reason)
	default:
		if h.Log != nil {
			h.Log.Error("chat request failed", "err", err)
		}
		writeError(w, http.StatusInternalServerError, "internal", "the request failed")
	}
}

// decode reads a small JSON body; anything else is invalid input.
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "message_too_long", "the body is too large")
			return false
		}
		writeError(w, http.StatusBadRequest, "invalid_input", "the body must be JSON")
		return false
	}
	return true
}
