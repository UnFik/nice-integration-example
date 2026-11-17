package sessionusecase

import (
	"context"

	sessiondto "github.com/nice-platform/example/sharedservice-oauth/backend/internal/app/session/dto"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
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

// ContextToken is the inventaris token of the active context of sess.
func (s *Service) ContextToken(ctx context.Context, sess domain.Session) (string, error) {
	if sess.Context == nil {
		return "", ErrNoContext
	}
	tok, err := s.NICE.ServiceToken(ctx, sess.Subject, *sess.Context)
	if err != nil {
		return "", s.end(ctx, sess, err)
	}
	return tok, nil
}

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
