package domain

import "errors"

// GrantError is a refusal of the NICE token endpoint, with the code and description NICE gave.
type GrantError struct {
	Code        string
	Description string
}

func (e *GrantError) Error() string { return "token exchange refused: " + e.Code }

// APIError is an answer of the NICE API other than 2xx, for the service map or the operations of a service.
type APIError struct {
	Status int
	Code   string
}

func (e *APIError) Error() string { return "the NICE API refused: " + e.Code }

// InvalidCallError is a call of an operation refused before anything was exchanged, e.g. a path parameter that is
// missing or is not one plain path segment. Reason names the parameter, never its value.
type InvalidCallError struct {
	Reason string
}

func (e *InvalidCallError) Error() string { return "invalid operation call: " + e.Reason }

var (
	// ErrTemporary means NICE did not answer the token exchange and a retry may work.
	ErrTemporary = errors.New("NICE did not answer the token exchange")
	// ErrUnreachable means the service itself did not answer.
	ErrUnreachable = errors.New("the service did not answer")
	// ErrIncompleteAnswer means the service stopped before it finished its answer.
	ErrIncompleteAnswer = errors.New("the service did not finish its answer")
)
