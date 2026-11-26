package chatusecase

import (
	"context"

	calldto "github.com/nice-platform/example/agent/backend/internal/app/call/dto"
	chatdto "github.com/nice-platform/example/agent/backend/internal/app/chat/dto"
	"github.com/nice-platform/example/agent/backend/internal/domain"
)

// CreateConversation starts a conversation of the user of c. An empty title is allowed: the first message names it.
func (s *Service) CreateConversation(ctx context.Context, c calldto.Caller, title string) (chatdto.Conversation, error) {
	if title != "" {
		t, err := domain.CleanTitle(title)
		if err != nil {
			return chatdto.Conversation{}, err
		}
		title = t
	}
	now := s.now()
	conv := chatdto.Conversation{ID: s.id(), Subject: c.Subject, Title: title, CreatedAt: now, UpdatedAt: now}
	if err := s.Conversations.Create(ctx, conv); err != nil {
		return chatdto.Conversation{}, err
	}
	return conv, nil
}
