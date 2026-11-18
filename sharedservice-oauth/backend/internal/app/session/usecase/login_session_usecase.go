package sessionusecase

import (
	"context"
	"net/url"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
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
	user, err := s.NICE.HandleCallback(ctx, pending.ID, query)
	if err != nil {
		return domain.Session{}, err
	}
	ids, err := randomIDs(2)
	if err != nil {
		return domain.Session{}, err
	}
	now := s.now()
	sess := domain.Session{ID: ids[0], Subject: pending.Subject, CSRFToken: ids[1], User: domain.User{ID: user}, Created: now, LastUsed: now}
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
// to, so the devices show at once; nil leaves the choice to the picker.
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
