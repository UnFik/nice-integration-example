package sessionusecase

import (
	"context"

	"github.com/nice-platform/example/agent/backend/internal/domain"
)

// ContextToken is the agen token of the active context of sess.
func (s *Service) ContextToken(ctx context.Context, sess domain.Session) (string, error) {
	if sess.Context == nil {
		return "", ErrNoContext
	}
	tok, err := s.NICE.ServiceToken(ctx, sess.Subject, *sess.Context)
	if err != nil {
		return "", s.end(ctx, sess, err)
	}
	if s.Stale == nil || !s.Stale(ctx, tok) {
		return tok, nil
	}
	// One new token only: if NICE issues a stale one again, the verifier
	// answers token_stale rather than this loop asking NICE without end.
	s.NICE.DropServiceToken(ctx, sess.Subject, *sess.Context, tok)
	if tok, err = s.NICE.ServiceToken(ctx, sess.Subject, *sess.Context); err != nil {
		return "", s.end(ctx, sess, err)
	}
	return tok, nil
}
