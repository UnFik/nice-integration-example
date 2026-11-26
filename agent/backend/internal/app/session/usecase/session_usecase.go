package sessionusecase

import (
	"context"

	"github.com/nice-platform/example/agent/backend/internal/domain"
)

// Session returns the signed-in session id and marks it used. A pending,
// unknown or idle session is ErrNoSession; an idle one is also signed out.
func (s *Service) Session(ctx context.Context, id string) (domain.Session, error) {
	if id == "" {
		return domain.Session{}, ErrNoSession
	}
	sess, ok, err := s.Store.Get(ctx, id)
	if err != nil {
		return domain.Session{}, err
	}
	if !ok || sess.Pending {
		return domain.Session{}, ErrNoSession
	}
	now := s.now()
	if s.expired(sess, now) {
		s.close(ctx, sess)
		return domain.Session{}, ErrNoSession
	}
	sess, ok, err = s.Store.Update(ctx, id, func(v *domain.Session) { v.LastUsed = now })
	if err != nil {
		return domain.Session{}, err
	}
	if !ok {
		return domain.Session{}, ErrNoSession
	}
	return sess, nil
}
