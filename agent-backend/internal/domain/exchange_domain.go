package domain

import "errors"

// GrantError is a refusal of the NICE token endpoint, with the code and description NICE gave.
type GrantError struct {
	Code        string
	Description string
}

func (e *GrantError) Error() string { return "token exchange refused: " + e.Code }

var (
	// ErrTemporary means NICE did not answer the token exchange and a retry may work.
	ErrTemporary = errors.New("NICE did not answer the token exchange")
	// ErrUnreachable means the service itself did not answer.
	ErrUnreachable = errors.New("the service did not answer")
	// ErrBuildRequest means the request to the service could not be built.
	ErrBuildRequest = errors.New("the request cannot be built")
	// ErrIncompleteAnswer means the service stopped before it finished its answer.
	ErrIncompleteAnswer = errors.New("the service did not finish its answer")
)
