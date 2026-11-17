package sessionusecase

import (
	"context"
	"errors"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

// Logout revokes the refresh token of sess at NICE, then removes the session.
// When NICE does not answer the session is kept and the error returned, so the
// user does not look signed out while the token still works.
func (s *Service) Logout(ctx context.Context, sess domain.Session) error {
	if err := s.NICE.Disconnect(ctx, sess.Subject); err != nil && !errors.Is(err, ErrSessionEnded) {
		return err
	}
	return s.Store.Delete(ctx, sess.ID)
}

// Sweep signs out the sessions not used within Idle and drops the sign-ins
// that were never completed. A session NICE cannot revoke now is kept for the
// next sweep.
func (s *Service) Sweep(ctx context.Context) {
	now := s.now()
	stale, err := s.Store.UsedBefore(ctx, now.Add(-min(s.Idle, PendingTTL)))
	if err != nil {
		return
	}
	for _, sess := range stale {
		if s.expired(sess, now) {
			s.close(ctx, sess)
		}
	}
}

func (s *Service) close(ctx context.Context, sess domain.Session) {
	if !sess.Pending {
		if err := s.NICE.Disconnect(ctx, sess.Subject); err != nil && !errors.Is(err, ErrSessionEnded) {
			return
		}
	}
	_ = s.Store.Delete(ctx, sess.ID)
}
