package sessionusecase

import (
	"context"

	sessiondto "github.com/nice-platform/example/agent/backend/internal/app/session/dto"
	"github.com/nice-platform/example/agent/backend/internal/domain"
)

// Contexts reads the organizations and projects of the user from NICE.
func (s *Service) Contexts(ctx context.Context, sess domain.Session) (sessiondto.Contexts, error) {
	signIn, err := s.NICE.SignInToken(ctx, sess.Subject)
	if err != nil {
		return sessiondto.Contexts{}, s.end(ctx, sess, err)
	}
	cs, err := s.NICE.Contexts(ctx, signIn)
	if err != nil {
		return sessiondto.Contexts{}, s.end(ctx, sess, err)
	}
	return cs, nil
}
