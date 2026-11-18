package sessionusecase

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

// PendingTTL is how long a started sign-in may take; niceclient keeps the
// state as long.
const PendingTTL = 10 * time.Minute

// Service implements the session use cases.
type Service struct {
	Store Store
	NICE  NICE
	// Idle ends a session not used for this long.
	Idle time.Duration
	// Stale reports whether the access change feed refuses token. The token
	// cache does not know the feed, so ContextToken asks NICE once for a token
	// it would refuse. Nil treats no token as stale.
	Stale func(ctx context.Context, token string) bool
	// Now is for tests; default time.Now.
	Now func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// randomID is 32 random bytes, URL safe: the session ID, the subject and the CSRF token.
func randomID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func randomIDs(n int) ([]string, error) {
	out := make([]string, n)
	for i := range out {
		id, err := randomID()
		if err != nil {
			return nil, err
		}
		out[i] = id
	}
	return out, nil
}

// expired reports whether sess may no longer be used at now.
func (s *Service) expired(sess domain.Session, now time.Time) bool {
	if sess.Pending {
		return now.Sub(sess.Created) > PendingTTL
	}
	return now.Sub(sess.LastUsed) > s.Idle
}

// end removes a session whose tokens NICE no longer honours. The refresh
// token is already gone, so nothing is revoked.
func (s *Service) end(ctx context.Context, sess domain.Session, err error) error {
	if errors.Is(err, ErrSessionEnded) {
		_ = s.Store.Delete(ctx, sess.ID)
	}
	return err
}
