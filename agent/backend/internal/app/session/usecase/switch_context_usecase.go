package sessionusecase

import (
	"context"

	"github.com/nice-platform/example/agent/backend/internal/domain"
)

// SwitchContext moves sess to c. The context changes only when NICE issued a
// token for c; a refusal leaves the session as it was.
func (s *Service) SwitchContext(ctx context.Context, sess domain.Session, c domain.Context) (domain.Session, error) {
	if c.Org == "" || c.Project == "" {
		return domain.Session{}, ErrInvalidContext
	}
	if _, err := s.NICE.ServiceToken(ctx, sess.Subject, c); err != nil {
		return domain.Session{}, s.end(ctx, sess, err)
	}
	next, ok, err := s.Store.Update(ctx, sess.ID, func(v *domain.Session) { v.Context = &c })
	if err != nil {
		return domain.Session{}, err
	}
	if !ok {
		return domain.Session{}, ErrNoSession
	}
	return next, nil
}
