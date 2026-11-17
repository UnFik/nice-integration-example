package callusecase

import (
	"context"

	calldto "github.com/nice-platform/example/agent-backend/internal/app/call/dto"
)

// Sender sends one request to a service with a token exchanged for key on behalf of the user who holds token. It is
// the only way the use case touches OAuth. Its errors are domain.GrantError, domain.ErrTemporary,
// domain.ErrUnreachable, domain.ErrBuildRequest, or domain.ErrIncompleteAnswer; any other error is a failed exchange.
type Sender interface {
	Send(ctx context.Context, token string, key calldto.Key, req calldto.Request) (calldto.Response, error)
}
