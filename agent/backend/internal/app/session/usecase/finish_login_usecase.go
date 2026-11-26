package sessionusecase

import (
	"context"
	"net/url"

	"github.com/nice-platform/example/agent/backend/internal/domain"
)

// FinishLogin completes the sign-in the pending session started and returns
// the new session. The pending ID is spent either way, and the new session
// gets a new ID, so an ID seen before the sign-in is worth nothing after it.
// The user's name, email and first context are best effort: a sign-in that
// cannot read them still succeeds, and the UI shows the picker.
func (s *Service) FinishLogin(ctx context.Context, pendingID string, query url.Values) (domain.Session, error) {
	pending, ok, err := s.Store.Get(ctx, pendingID)
	if err != nil {
		return domain.Session{}, err
	}
	if !ok || !pending.Pending || s.expired(pending, s.now()) {
		return domain.Session{}, ErrStateMismatch
	}
	_ = s.Store.Delete(ctx, pendingID)
	user, idToken, err := s.NICE.HandleCallback(ctx, pending.ID, query)
	if err != nil {
		return domain.Session{}, err
	}
	ids, err := randomIDs(2)
	if err != nil {
		return domain.Session{}, err
	}
	now := s.now()
	sess := domain.Session{ID: ids[0], Subject: pending.Subject, CSRFToken: ids[1], User: domain.User{ID: user}, IDToken: idToken, Created: now, LastUsed: now}
	if signIn, err := s.NICE.SignInToken(ctx, sess.Subject); err == nil {
		if info, err := s.NICE.UserInfo(ctx, signIn); err == nil {
			sess.User.Name, sess.User.Email = info.Name, info.Email
		}
		sess.Context = s.firstContext(ctx, sess.Subject, signIn)
	}
	if err := s.Store.Create(ctx, sess); err != nil {
		return domain.Session{}, err
	}
	return sess, nil
}

// firstContext picks the first project of the organization the user signed in
// to, so the chat works at once; nil leaves the choice to the picker.
func (s *Service) firstContext(ctx context.Context, subject, signIn string) *domain.Context {
	cs, err := s.NICE.Contexts(ctx, signIn)
	if err != nil {
		return nil
	}
	for _, o := range cs.Organizations {
		if o.ID != cs.CurrentOrganizationID || len(o.Projects) == 0 {
			continue
		}
		c := domain.Context{Org: o.ID, Project: o.Projects[0].ID}
		if _, err := s.NICE.ServiceToken(ctx, subject, c); err != nil {
			return nil
		}
		return &c
	}
	return nil
}
