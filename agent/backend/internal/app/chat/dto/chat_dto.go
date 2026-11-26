// Package chatdto holds the data that crosses the boundary of the chat use case. Messages are kept in a neutral
// form, so the store does not change with the model provider.
package chatdto

import (
	"encoding/json"
	"time"

	calldto "github.com/nice-platform/example/agent/backend/internal/app/call/dto"
)

// Conversation belongs to one user, the sub of the session token.
type Conversation struct {
	ID        string
	Subject   string
	Title     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Message is one entry of a conversation. Seq orders the messages of a conversation from 1.
type Message struct {
	ID             string
	ConversationID string
	Seq            int
	Role           string // domain.Role*
	Parts          []Part
	CreatedAt      time.Time
}

// Part is one piece of a message: text, a tool call of the assistant, or the result of one. Exactly one of Text,
// Call, and Result is set, as Kind says. It is stored as JSON.
type Part struct {
	Kind   string      `json:"kind"` // domain.Part*
	Text   string      `json:"text,omitempty"`
	Call   *ToolCall   `json:"call,omitempty"`
	Result *ToolResult `json:"result,omitempty"`
}

// ToolCall is a call the model asked for. ApprovalState and ApprovalMessage are set while Status is
// domain.CallAwaitingApproval: the state the call use case issued, and the message built from the catalog.
type ToolCall struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Input           json.RawMessage `json:"input"`
	Status          string          `json:"status,omitempty"` // domain.Call*
	ApprovalState   string          `json:"approval_state,omitempty"`
	ApprovalMessage string          `json:"approval_message,omitempty"`
}

// ToolResult answers the call with the same ID. Content is the JSON the MCP tool returns as structuredContent, or
// {"error": code, "message": text} when IsError.
type ToolResult struct {
	CallID  string          `json:"call_id"`
	Content json.RawMessage `json:"content"`
	IsError bool            `json:"is_error,omitempty"`
}

// ToolSpec describes a tool to the model: the same name, description, and input schema as the MCP tool.
type ToolSpec struct {
	Name        string
	Description string
	InputSchema json.RawMessage
}

// ModelRequest is one call of the model: the system prompt, the conversation so far, and the tools.
type ModelRequest struct {
	System   string
	Messages []Message
	Tools    []ToolSpec
}

// ModelReply is the answer of the model: text, tool calls, or both. No call means the turn ends.
type ModelReply struct {
	Text  string
	Calls []ToolCall
}

// ToolOutcome is what running one tool call gave: its result, or a request for the user's approval.
type ToolOutcome struct {
	Content  json.RawMessage
	IsError  bool
	Approval *calldto.ApprovalRequest
}

// Event is one step of a turn as the dashboard sees it, sent after it is stored. Type is one of assistant,
// tool_call, tool_result, approval, done, error.
type Event struct {
	Type    string   `json:"type"`
	Message *Message `json:"-"`
	// CallID and Text are set for approval (the call that waits and the catalog message) and error (a code and text).
	CallID string `json:"call_id,omitempty"`
	Code   string `json:"code,omitempty"`
	Text   string `json:"text,omitempty"`
}
