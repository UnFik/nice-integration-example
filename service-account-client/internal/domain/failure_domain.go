package domain

import "errors"

// GrantError is a refusal of the NICE token endpoint, with the code and description NICE gave.
type GrantError struct {
	Code        string
	Description string
	// Permanent is true when NICE refused the credentials themselves, so trying again cannot work.
	Permanent bool
}

func (e *GrantError) Error() string { return "token refused: " + e.Code }

// Is makes errors.Is(err, ErrKeyRefused) true for a permanent refusal.
func (e *GrantError) Is(target error) bool { return target == ErrKeyRefused && e.Permanent }

// ServiceError is an answer of nadi that refused the request, with the code and description it gave.
type ServiceError struct {
	Status      int
	Code        string
	Description string
}

func (e *ServiceError) Error() string { return "nadi refused the request: " + e.Description }

var (
	// ErrKeyRefused means NICE refused the API key for good: the client must stop until the key is replaced.
	ErrKeyRefused = errors.New("NICE refused the API key")
	// ErrTemporary means NICE did not answer the token request and a retry may work.
	ErrTemporary = errors.New("NICE did not answer the token request")
	// ErrUnreachable means nadi itself did not answer.
	ErrUnreachable = errors.New("nadi did not answer")
	// ErrBadAnswer means nadi answered something the client cannot read.
	ErrBadAnswer = errors.New("nadi answered something unexpected")
)
