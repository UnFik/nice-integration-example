package sessionusecase

import (
	"context"
	"errors"

	"github.com/nice-platform/example/agent/backend/internal/domain"
)

// Logout revokes the refresh token of sess at NICE, then removes the session.
// When NICE does not answer the session is kept and the error returned, so the
// user does not look signed out while the token still works. It also returns
// where the browser may go next to end its NICE session; empty when that
// address cannot be built: the user is signed out of agen all the same.
func (s *Service) Logout(ctx context.Context, sess domain.Session) (endSessionURL string, err error) {
	if err := s.NICE.Disconnect(ctx, sess.Subject); err != nil && !errors.Is(err, ErrSessionEnded) {
		return "", err
	}
	if err := s.Store.Delete(ctx, sess.ID); err != nil {
		return "", err
	}
	return s.EndSessionURL(sess.IDToken), nil
}
