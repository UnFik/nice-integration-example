package chatusecase

import (
	"context"

	calldto "github.com/nice-platform/example/agent/backend/internal/app/call/dto"
	chatdto "github.com/nice-platform/example/agent/backend/internal/app/chat/dto"
)

// ListConversations returns the conversations of the user of c, the latest change first.
func (s *Service) ListConversations(ctx context.Context, c calldto.Caller) ([]chatdto.Conversation, error) {
	return s.Conversations.List(ctx, c.Subject)
}
