package chatusecase

import (
	"context"

	calldto "github.com/nice-platform/example/agent/backend/internal/app/call/dto"
)

// DeleteConversation removes a conversation of the user of c and its messages.
func (s *Service) DeleteConversation(ctx context.Context, c calldto.Caller, id string) error {
	return s.Conversations.Delete(ctx, c.Subject, id)
}
