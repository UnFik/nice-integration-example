package callusecase

import (
	"context"

	calldto "github.com/nice-platform/example/agent/backend/internal/app/call/dto"
)

// Discoverer reads what NICE says the user of token may call: the service map, and the operations of one service.
// Its errors are domain.APIError, domain.GrantError, domain.ErrTemporary, or domain.ErrUnreachable.
type Discoverer interface {
	Services(ctx context.Context, token string) (calldto.ServiceMap, error)
	// Operations reads the operations of slug for user, the subject of token, which keys what the adapter keeps.
	Operations(ctx context.Context, token, user, slug string) (calldto.ServiceOperations, error)
}

// Operator sends one call of an operation with a token exchanged on behalf of the user who holds token. It is the
// only way the use case reaches a service. Its errors are domain.InvalidCallError (nothing was exchanged),
// domain.GrantError, domain.ErrTemporary, domain.ErrUnreachable, or domain.ErrIncompleteAnswer.
type Operator interface {
	CallOperation(ctx context.Context, token string, call calldto.OperationCall) (calldto.OperationResult, error)
}
