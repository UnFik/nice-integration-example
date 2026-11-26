package chatusecase

import (
	"context"

	calldto "github.com/nice-platform/example/agent/backend/internal/app/call/dto"
	chatdto "github.com/nice-platform/example/agent/backend/internal/app/chat/dto"
)

// GetConversation returns one conversation of the user of c and its messages. Another user's conversation is
// domain.ErrConversationNotFound.
func (s *Service) GetConversation(ctx context.Context, c calldto.Caller, id string) (chatdto.Conversation, []chatdto.Message, error) {
	return s.Conversations.Get(ctx, c.Subject, id)
}
