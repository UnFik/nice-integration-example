package chat

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/nice-platform/example/agent/backend/internal/adapter/rest/chat/dto"
	chatdto "github.com/nice-platform/example/agent/backend/internal/app/chat/dto"
)

// stream writes the events of a turn as server-sent events. The response starts with the first event, so an error
// returned before the turn starts can still be an ordinary JSON answer. Writes after the browser left fail quietly:
// the turn goes on and its steps are stored.
type stream struct {
	mu      sync.Mutex
	w       http.ResponseWriter
	started bool
}

func (s *stream) emit(e chatdto.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		h := s.w.Header()
		h.Set("Content-Type", "text/event-stream")
		h.Set("Cache-Control", "no-store")
		h.Set("X-Accel-Buffering", "no")
		s.w.WriteHeader(http.StatusOK)
		s.started = true
	}
	data, _ := json.Marshal(eventData(e))
	_, _ = s.w.Write([]byte("event: " + e.Type + "\ndata: " + string(data) + "\n\n"))
	if f, ok := s.w.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *stream) hasStarted() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.started
}

func eventData(e chatdto.Event) any {
	switch e.Type {
	case "assistant", "tool_result":
		var m dto.Message
		if e.Message != nil {
			m = dto.FromMessage(*e.Message)
		}
		return map[string]any{"message": m}
	case "tool_call":
		return map[string]string{"call_id": e.CallID, "name": e.Text}
	case "approval":
		return map[string]string{"call_id": e.CallID, "message": e.Text}
	case "done":
		return map[string]string{"reason": e.Code}
	default:
		return map[string]string{"error": e.Code, "message": e.Text}
	}
}
