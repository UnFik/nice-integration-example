package chatusecase

import (
	"context"
	"time"

	calldto "github.com/nice-platform/example/agent/backend/internal/app/call/dto"
	chatdto "github.com/nice-platform/example/agent/backend/internal/app/chat/dto"
)

// Conversations keeps the conversations of each user. Every method that takes subject and an ID answers
// domain.ErrConversationNotFound when the conversation does not exist or belongs to another user, so a caller learns
// nothing about the conversations of others.
type Conversations interface {
	// List returns the conversations of subject, the latest UpdatedAt first.
	List(ctx context.Context, subject string) ([]chatdto.Conversation, error)
	// Get returns one conversation and its messages in Seq order.
	Get(ctx context.Context, subject, id string) (chatdto.Conversation, []chatdto.Message, error)
	Create(ctx context.Context, c chatdto.Conversation) error
	Rename(ctx context.Context, subject, id, title string, at time.Time) error
	// Delete removes the conversation and its messages.
	Delete(ctx context.Context, subject, id string) error
	// Append stores m as the next message of the conversation (it sets Seq) and moves UpdatedAt to m.CreatedAt.
	Append(ctx context.Context, subject, conversationID string, m chatdto.Message) (chatdto.Message, error)
	// UpdateCall replaces the tool call part with call.ID in the conversation, keeping its message and Seq. It answers
	// domain.ErrCallNotPending when no part of the conversation holds that call.
	UpdateCall(ctx context.Context, subject, conversationID string, call chatdto.ToolCall) error
}

// Model is the chat model. An error wraps domain.ErrModelUnavailable or domain.ErrModelRefused and never holds the
// key of the provider.
type Model interface {
	Next(ctx context.Context, req chatdto.ModelRequest) (chatdto.ModelReply, error)
}

// Tools runs the tools of the agent for the chat: the same use cases, inputs, and results as the MCP tools.
type Tools interface {
	Specs() []chatdto.ToolSpec
	// Run runs one call for caller. ap carries the user's answer when the call was waiting for approval; nil for a
	// first run.
	Run(ctx context.Context, caller calldto.Caller, call chatdto.ToolCall, ap *calldto.Approval) chatdto.ToolOutcome
}
