package sessionusecase

import (
	"context"

	"github.com/nice-platform/example/agent/backend/internal/domain"
)

// StartLogin creates a pending session and returns it with the address of
// the NICE sign-in. A pending session of the same browser (previous) is dropped.
func (s *Service) StartLogin(ctx context.Context, previous, prompt string) (domain.Session, string, error) {
	if previous != "" {
		if old, ok, err := s.Store.Get(ctx, previous); err == nil && ok && old.Pending {
			_ = s.Store.Delete(ctx, previous)
		}
	}
	ids, err := randomIDs(3)
	if err != nil {
		return domain.Session{}, "", err
	}
	now := s.now()
	sess := domain.Session{ID: ids[0], Subject: ids[1], CSRFToken: ids[2], Pending: true, Created: now, LastUsed: now}
	authURL, err := s.NICE.AuthorizeURL(ctx, sess.Subject, sess.ID, prompt)
	if err != nil {
		return domain.Session{}, "", err
	}
	if err := s.Store.Create(ctx, sess); err != nil {
		return domain.Session{}, "", err
	}
	return sess, authURL, nil
}
