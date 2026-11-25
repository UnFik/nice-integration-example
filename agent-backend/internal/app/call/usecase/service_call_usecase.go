// Package callusecase finds what the user of an MCP request may call, and runs one call of an operation on the
// user's behalf.
package callusecase

import (
	"log/slog"
	"sync"

	"github.com/nice-platform/example/agent-backend/internal/app/call/internal/approval"
)

// Service runs discovery and calls. ClientVars names the settings of the agent's own client, for the message of a
// refused client.
type Service struct {
	Discoverer Discoverer
	Operator   Operator
	ClientVars string
	Log        *slog.Logger

	once      sync.Once
	approvals *approval.Book
}

// book is the approval book of this process, made on first use.
func (s *Service) book() *approval.Book {
	s.once.Do(func() { s.approvals = approval.New(nil) })
	return s.approvals
}
