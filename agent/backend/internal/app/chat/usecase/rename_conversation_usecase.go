package chatusecase

import (
	"context"

	calldto "github.com/nice-platform/example/agent/backend/internal/app/call/dto"
	"github.com/nice-platform/example/agent/backend/internal/domain"
)

// RenameConversation sets the title of a conversation of the user of c.
func (s *Service) RenameConversation(ctx context.Context, c calldto.Caller, id, title string) error {
	t, err := domain.CleanTitle(title)
	if err != nil {
		return err
	}
	return s.Conversations.Rename(ctx, c.Subject, id, t, s.now())
}
