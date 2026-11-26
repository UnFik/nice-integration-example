package turn

import (
	"encoding/json"

	chatdto "github.com/nice-platform/example/agent/backend/internal/app/chat/dto"
	"github.com/nice-platform/example/agent/backend/internal/domain"
)

// ForModel shapes the stored history for a model call. Results of one assistant message are stored one tool message
// each, as they come, so consecutive tool messages are merged: providers expect every result of a reply together.
// A call that still waits for approval gets no result and is left out, with its result, of what the model reads.
func ForModel(msgs []chatdto.Message) []chatdto.Message {
	pending := map[string]bool{}
	for _, m := range msgs {
		for _, p := range m.Parts {
			if p.Kind == domain.PartToolCall && p.Call != nil && p.Call.Status == domain.CallAwaitingApproval {
				pending[p.Call.ID] = true
			}
		}
	}
	var out []chatdto.Message
	for _, m := range msgs {
		var parts []chatdto.Part
		for _, p := range m.Parts {
			if p.Kind == domain.PartToolCall && p.Call != nil && pending[p.Call.ID] {
				continue
			}
			parts = append(parts, p)
		}
		if len(parts) == 0 {
			continue
		}
		m.Parts = parts
		if n := len(out); n > 0 && m.Role == domain.RoleTool && out[n-1].Role == domain.RoleTool {
			out[n-1].Parts = append(append([]chatdto.Part(nil), out[n-1].Parts...), parts...)
			continue
		}
		out = append(out, m)
	}
	return out
}

// Pending returns the calls of msgs that wait for approval, in order.
func Pending(msgs []chatdto.Message) []chatdto.ToolCall {
	var out []chatdto.ToolCall
	for _, m := range msgs {
		for _, p := range m.Parts {
			if p.Kind == domain.PartToolCall && p.Call != nil && p.Call.Status == domain.CallAwaitingApproval {
				out = append(out, *p.Call)
			}
		}
	}
	return out
}

// ErrorContent is the content of a failed tool result: the code and the text the model and the user read.
func ErrorContent(code, message string) json.RawMessage {
	b, _ := json.Marshal(map[string]string{"error": code, "message": message})
	return b
}
