// Package dto is the JSON of the chat API the dashboard reads. It leaves out what only the backend needs, such as
// the approval state.
package dto

import (
	"encoding/json"
	"time"

	chatdto "github.com/nice-platform/example/agent/backend/internal/app/chat/dto"
)

type Conversation struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ConversationList struct {
	Conversations []Conversation `json:"conversations"`
	// ChatEnabled is false when the agent has no model: history can be read, nothing can be sent.
	ChatEnabled bool `json:"chat_enabled"`
}

type ConversationDetail struct {
	Conversation Conversation `json:"conversation"`
	Messages     []Message    `json:"messages"`
}

type Message struct {
	ID        string    `json:"id"`
	Seq       int       `json:"seq"`
	Role      string    `json:"role"`
	Parts     []Part    `json:"parts"`
	CreatedAt time.Time `json:"created_at"`
}

type Part struct {
	Kind   string      `json:"kind"`
	Text   string      `json:"text,omitempty"`
	Call   *ToolCall   `json:"call,omitempty"`
	Result *ToolResult `json:"result,omitempty"`
}

type ToolCall struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Input           json.RawMessage `json:"input"`
	Status          string          `json:"status,omitempty"`
	ApprovalMessage string          `json:"approval_message,omitempty"`
}

type ToolResult struct {
	CallID  string          `json:"call_id"`
	Content json.RawMessage `json:"content"`
	IsError bool            `json:"is_error,omitempty"`
}

type CreateRequest struct {
	Title string `json:"title"`
}

type RenameRequest struct {
	Title string `json:"title"`
}

type SendRequest struct {
	Text string `json:"text"`
}

type ApprovalRequest struct {
	Approve *bool `json:"approve"`
}

type Error struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func FromConversation(c chatdto.Conversation) Conversation {
	return Conversation{ID: c.ID, Title: c.Title, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt}
}

func FromMessage(m chatdto.Message) Message {
	out := Message{ID: m.ID, Seq: m.Seq, Role: m.Role, CreatedAt: m.CreatedAt, Parts: []Part{}}
	for _, p := range m.Parts {
		q := Part{Kind: p.Kind, Text: p.Text}
		if p.Call != nil {
			in := p.Call.Input
			if len(in) == 0 {
				in = json.RawMessage(`{}`)
			}
			q.Call = &ToolCall{ID: p.Call.ID, Name: p.Call.Name, Input: in, Status: p.Call.Status, ApprovalMessage: p.Call.ApprovalMessage}
		}
		if p.Result != nil {
			c := p.Result.Content
			if len(c) == 0 {
				c = json.RawMessage(`null`)
			}
			q.Result = &ToolResult{CallID: p.Result.CallID, Content: c, IsError: p.Result.IsError}
		}
		out.Parts = append(out.Parts, q)
	}
	return out
}
