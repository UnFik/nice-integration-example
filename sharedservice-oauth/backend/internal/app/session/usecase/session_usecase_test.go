package sessionusecase_test

import (
	"context"
	"errors"
	"net/url"
	"slices"
	"sync"
	"testing"
	"time"

	sessionmemory "github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/memory/session"
	sessiondto "github.com/nice-platform/example/sharedservice-oauth/backend/internal/app/session/dto"
	sessionusecase "github.com/nice-platform/example/sharedservice-oauth/backend/internal/app/session/usecase"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

// fakeNICE records what the use cases ask of NICE.
type fakeNICE struct {
	mu           sync.Mutex
	authorized   map[string]string // session -> subject
	tokenErr     error
	disconnected []string
	disconnect   error
	// tokens are handed out by ServiceToken in order until one is left, which
	// is then handed out again; empty hands out "service-token".
	tokens  []string
	fetches int
	dropped []string
}

func (f *fakeNICE) AuthorizeURL(_ context.Context, subject, session, _ string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.authorized == nil {
		f.authorized = map[string]string{}
	}
	f.authorized[session] = subject
	return "http://nice.test/oauth/authorize?state=x", nil
}

func (f *fakeNICE) HandleCallback(_ context.Context, session string, q url.Values) (string, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.authorized[session]; !ok || q.Get("state") != "x" {
		return "", "", sessionusecase.ErrStateMismatch
	}
	if e := q.Get("error"); e != "" {
		return "", "", &sessionusecase.RefusedError{Code: e}
	}
	return "usr_1", "id-token", nil
}

func (f *fakeNICE) EndSessionURL(idTokenHint string) (string, error) {
	return "http://nice.test/oauth/end_session?hint=" + idTokenHint, nil
}

func (f *fakeNICE) ServiceToken(context.Context, string, domain.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fetches++
	if f.tokenErr != nil {
		return "", f.tokenErr
	}
	if len(f.tokens) == 0 {
		return "service-token", nil
	}
	tok := f.tokens[0]
	if len(f.tokens) > 1 {
		f.tokens = f.tokens[1:]
	}
	return tok, nil
}

func (f *fakeNICE) DropServiceToken(_ context.Context, _ string, _ domain.Context, token string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dropped = append(f.dropped, token)
}

func (f *fakeNICE) SignInToken(context.Context, string) (string, error) { return "sign-in", nil }

func (f *fakeNICE) UserInfo(context.Context, string) (sessiondto.UserInfo, error) {
	return sessiondto.UserInfo{Name: "Budi", Email: "budi@example.com"}, nil
}

func (f *fakeNICE) Contexts(context.Context, string) (sessiondto.Contexts, error) {
	return sessiondto.Contexts{CurrentOrganizationID: "org_a", Organizations: []sessiondto.Organization{
		{ID: "org_a", Projects: []sessiondto.Project{{ID: "prj_p"}, {ID: "prj_q"}}}}}, nil
}

func (f *fakeNICE) Disconnect(_ context.Context, subject string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.disconnect != nil {
		return f.disconnect
	}
	f.disconnected = append(f.disconnected, subject)
	return nil
}

type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.now }
func (c *clock) add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func newService() (*sessionusecase.Service, *fakeNICE, *sessionmemory.Store, *clock) {
	n, st, c := &fakeNICE{}, sessionmemory.NewStore(), &clock{now: time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC)}
	return &sessionusecase.Service{Store: st, NICE: n, Idle: 8 * time.Hour, Now: c.Now}, n, st, c
}

func signIn(t *testing.T, s *sessionusecase.Service) domain.Session {
	t.Helper()
	ctx := context.Background()
	pending, _, err := s.StartLogin(ctx, "", "")
	if err != nil {
		t.Fatal(err)
	}
	sess, err := s.FinishLogin(ctx, pending.ID, url.Values{"state": {"x"}, "code": {"c"}})
	if err != nil {
		t.Fatal(err)
	}
	return sess
}

func TestSignInCreatesASessionApartFromItsSubject(t *testing.T) {
	s, n, st, _ := newService()
	ctx := context.Background()
	pending, _, err := s.StartLogin(ctx, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !pending.Pending || pending.Subject == "" || pending.Subject == pending.ID || n.authorized[pending.ID] != pending.Subject {
		t.Fatalf("pending = %+v", pending)
	}
	if _, err := s.Session(ctx, pending.ID); !errors.Is(err, sessionusecase.ErrNoSession) {
		t.Fatalf("a pending sign-in used as a session: %v", err)
	}
	sess, err := s.FinishLogin(ctx, pending.ID, url.Values{"state": {"x"}})
	if err != nil {
		t.Fatal(err)
	}
	if sess.ID == pending.ID || sess.Subject != pending.Subject || sess.CSRFToken == "" || sess.CSRFToken == pending.CSRFToken {
		t.Fatalf("session = %+v, want a new ID and CSRF token for the same subject", sess)
	}
	if sess.User != (domain.User{ID: "usr_1", Name: "Budi", Email: "budi@example.com"}) ||
		sess.Context == nil || *sess.Context != (domain.Context{Org: "org_a", Project: "prj_p"}) {
		t.Fatalf("session = %+v", sess)
	}
	if got, err := s.Session(ctx, sess.ID); err != nil || got.ID != sess.ID {
		t.Fatalf("read = %+v, %v", got, err)
	}
	if _, err := s.FinishLogin(ctx, pending.ID, url.Values{"state": {"x"}}); !errors.Is(err, sessionusecase.ErrStateMismatch) {
		t.Fatalf("a pending sign-in used twice: %v", err)
	}
	if st.Len() != 1 {
		t.Fatalf("%d sessions, want 1", st.Len())
	}
}

func TestRefusedSignInLeavesNoSession(t *testing.T) {
	s, _, st, _ := newService()
	ctx := context.Background()
	pending, _, _ := s.StartLogin(ctx, "", "")
	_, err := s.FinishLogin(ctx, pending.ID, url.Values{"state": {"x"}, "error": {"access_denied"}})
	var refused *sessionusecase.RefusedError
	if !errors.As(err, &refused) || refused.Code != "access_denied" || st.Len() != 0 {
		t.Fatalf("err = %v, %d sessions", err, st.Len())
	}
}

func TestSessionEndsAfterEightIdleHours(t *testing.T) {
	s, n, st, c := newService()
	ctx := context.Background()
	sess := signIn(t, s)
	c.add(7 * time.Hour)
	if _, err := s.Session(ctx, sess.ID); err != nil {
		t.Fatalf("after 7 hours: %v", err)
	}
	// Use moves the deadline: 7 more hours after the last use is still in.
	c.add(7 * time.Hour)
	if _, err := s.Session(ctx, sess.ID); err != nil {
		t.Fatalf("7 hours after the last use: %v", err)
	}
	c.add(8*time.Hour + time.Second)
	if _, err := s.Session(ctx, sess.ID); !errors.Is(err, sessionusecase.ErrNoSession) {
		t.Fatalf("after 8 idle hours: %v", err)
	}
	if st.Len() != 0 || len(n.disconnected) != 1 || n.disconnected[0] != sess.Subject {
		t.Fatalf("%d sessions, disconnected %v; want the refresh token revoked", st.Len(), n.disconnected)
	}
}

func TestSweepSignsOutIdleSessionsAndStaleSignIns(t *testing.T) {
	s, n, st, c := newService()
	ctx := context.Background()
	idle := signIn(t, s)
	_, _, _ = s.StartLogin(ctx, "", "")
	c.add(8*time.Hour + time.Minute)
	active := signIn(t, s)
	s.Sweep(ctx)
	if st.Len() != 1 || len(n.disconnected) != 1 || n.disconnected[0] != idle.Subject {
		t.Fatalf("%d sessions, disconnected %v", st.Len(), n.disconnected)
	}
	if _, err := s.Session(ctx, active.ID); err != nil {
		t.Fatal(err)
	}
}

func TestLogout(t *testing.T) {
	s, n, st, _ := newService()
	ctx := context.Background()
	sess := signIn(t, s)
	down := &sessionusecase.UnavailableError{Err: errors.New("down")}
	n.disconnect = down
	if end, err := s.Logout(ctx, sess); !errors.As(err, &down) || st.Len() != 1 || end != "" {
		t.Fatalf("logout while NICE is down = %q, %v, %d sessions; want the session kept", end, err, st.Len())
	}
	n.disconnect = nil
	end, err := s.Logout(ctx, sess)
	if err != nil || st.Len() != 0 {
		t.Fatalf("logout = %v, %d sessions", err, st.Len())
	}
	// The ID token of the sign-in goes back to NICE as the hint.
	if end != "http://nice.test/oauth/end_session?hint=id-token" {
		t.Fatalf("end-session address = %q", end)
	}
}

func TestSwitchContext(t *testing.T) {
	s, n, st, _ := newService()
	ctx := context.Background()
	sess := signIn(t, s)
	n.tokenErr = &sessionusecase.RefusedError{Code: "access_denied"}
	if _, err := s.SwitchContext(ctx, sess, domain.Context{Org: "org_b", Project: "prj_r"}); err == nil {
		t.Fatal("a refused switch succeeded")
	}
	if got, _ := s.Session(ctx, sess.ID); *got.Context != (domain.Context{Org: "org_a", Project: "prj_p"}) {
		t.Fatalf("context after a refusal = %+v", got.Context)
	}
	n.tokenErr = nil
	next, err := s.SwitchContext(ctx, sess, domain.Context{Org: "org_b", Project: "prj_r"})
	if err != nil || *next.Context != (domain.Context{Org: "org_b", Project: "prj_r"}) {
		t.Fatalf("switch = %+v, %v", next.Context, err)
	}
	if _, err := s.SwitchContext(ctx, sess, domain.Context{Org: "org_b"}); !errors.Is(err, sessionusecase.ErrInvalidContext) {
		t.Fatalf("switch without project = %v", err)
	}
	n.tokenErr = sessionusecase.ErrSessionEnded
	if _, err := s.ContextToken(ctx, next); !errors.Is(err, sessionusecase.ErrSessionEnded) || st.Len() != 0 {
		t.Fatalf("session ended by NICE = %v, %d sessions; want it removed", err, st.Len())
	}
}

// staleTokens treats the named tokens as refused by the access change feed.
func staleTokens(names ...string) func(context.Context, string) bool {
	return func(_ context.Context, tok string) bool {
		return slices.Contains(names, tok)
	}
}

func TestContextTokenRenewsAStaleTokenOnce(t *testing.T) {
	cases := []struct {
		name        string
		tokens      []string
		stale       []string
		tokenErr    error
		want        string
		wantFetches int
		wantDropped []string
	}{
		{name: "fresh", tokens: []string{"t1"}, want: "t1", wantFetches: 1},
		{name: "stale then fresh", tokens: []string{"t1", "t2"}, stale: []string{"t1"}, want: "t2", wantFetches: 2,
			wantDropped: []string{"t1"}},
		{name: "stale twice", tokens: []string{"t1", "t2"}, stale: []string{"t1", "t2"}, want: "t2", wantFetches: 2,
			wantDropped: []string{"t1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, n, _, _ := newService()
			ctx := context.Background()
			sess := signIn(t, s)
			s.Stale = staleTokens(tc.stale...)
			n.mu.Lock()
			n.tokens, n.fetches = tc.tokens, 0
			n.mu.Unlock()
			got, err := s.ContextToken(ctx, sess)
			if err != nil || got != tc.want {
				t.Fatalf("token = %q, %v; want %q", got, err, tc.want)
			}
			if n.fetches != tc.wantFetches || !slices.Equal(n.dropped, tc.wantDropped) {
				t.Fatalf("fetches = %d, dropped = %v; want %d, %v", n.fetches, n.dropped, tc.wantFetches, tc.wantDropped)
			}
		})
	}
}

func TestContextTokenRenewalRefusedByNICE(t *testing.T) {
	s, n, st, _ := newService()
	ctx := context.Background()
	sess := signIn(t, s)
	s.Stale = func(context.Context, string) bool {
		// NICE refuses the new token: the user may no longer use the context.
		n.tokenErr = &sessionusecase.RefusedError{Code: "access_denied"}
		return true
	}
	_, err := s.ContextToken(ctx, sess)
	var refused *sessionusecase.RefusedError
	if !errors.As(err, &refused) || refused.Code != "access_denied" {
		t.Fatalf("err = %v, want access_denied", err)
	}
	if got, _ := s.Session(ctx, sess.ID); st.Len() != 1 || *got.Context != *sess.Context {
		t.Fatalf("%d sessions, context %+v; want the session and its context kept", st.Len(), got.Context)
	}
}
