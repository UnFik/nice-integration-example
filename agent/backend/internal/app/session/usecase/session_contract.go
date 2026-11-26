// Package sessionusecase holds the use cases of the browser session: sign-in,
// the active context, switching it and signing out. NICE and the session
// storage are reached through the ports below.
package sessionusecase

import (
	"context"
	"errors"
	"net/url"
	"time"

	sessiondto "github.com/nice-platform/example/agent/backend/internal/app/session/dto"
	"github.com/nice-platform/example/agent/backend/internal/domain"
)

// Store keeps the browser sessions.
type Store interface {
	Create(ctx context.Context, s domain.Session) error
	Get(ctx context.Context, id string) (domain.Session, bool, error)
	// Update changes an existing session under a lock; it reports false when
	// the session is gone, so a sign-out is never undone by a late update.
	Update(ctx context.Context, id string, change func(*domain.Session)) (domain.Session, bool, error)
	Delete(ctx context.Context, id string) error
	// UsedBefore returns the sessions last used before t.
	UsedBefore(ctx context.Context, t time.Time) ([]domain.Session, error)
}

// NICE is what the session use cases need from NICE. subject is the
// niceauth/credentials subject of a session, never its cookie value. The errors are
// those below.
type NICE interface {
	// AuthorizeURL starts the sign-in of subject for the local session.
	AuthorizeURL(ctx context.Context, subject, session, prompt string) (string, error)
	// HandleCallback completes it and returns the NICE user (usr_...) and the
	// ID token of the sign-in, empty when NICE gave none.
	HandleCallback(ctx context.Context, session string, query url.Values) (user, idToken string, err error)
	// ServiceToken is the agen token of subject in c.
	ServiceToken(ctx context.Context, subject string, c domain.Context) (string, error)
	// DropServiceToken forgets token if it is still the one kept for subject in
	// c, so the next ServiceToken asks NICE for a new one.
	DropServiceToken(ctx context.Context, subject string, c domain.Context, token string)
	// SignInToken is the token of the sign-in itself, for userinfo and for the
	// exchange that reads the contexts.
	SignInToken(ctx context.Context, subject string) (string, error)
	UserInfo(ctx context.Context, signIn string) (sessiondto.UserInfo, error)
	Contexts(ctx context.Context, signIn string) (sessiondto.Contexts, error)
	// Disconnect revokes the refresh token of subject at NICE.
	Disconnect(ctx context.Context, subject string) error
	// EndSessionURL is where the browser ends its NICE session (RP-initiated
	// logout); idTokenHint is optional.
	EndSessionURL(idTokenHint string) (string, error)
}

var (
	// ErrNoSession is a request without a usable session: sign in.
	ErrNoSession = errors.New("no session")
	// ErrSessionEnded is a session NICE ended (invalid_grant): sign in again.
	ErrSessionEnded = errors.New("the NICE session ended")
	// ErrStateMismatch is a callback that does not belong to this browser session.
	ErrStateMismatch = errors.New("the callback does not belong to this session")
	// ErrNoContext is a session without an active organization and project.
	ErrNoContext = errors.New("no active context")
	// ErrInvalidContext is a context without an organization or a project.
	ErrInvalidContext = errors.New("a context needs an organization and a project")
)

// RefusedError is a refusal of NICE that leaves the session alive, such as
// access_denied or invalid_scope for an organization or project.
type RefusedError struct {
	Code        string
	Description string
}

func (e *RefusedError) Error() string { return "NICE refused: " + e.Code + " " + e.Description }

// UnavailableError is a failure to get an answer from NICE; worth trying again.
type UnavailableError struct{ Err error }

func (e *UnavailableError) Error() string { return "NICE did not answer: " + e.Err.Error() }
func (e *UnavailableError) Unwrap() error { return e.Err }
