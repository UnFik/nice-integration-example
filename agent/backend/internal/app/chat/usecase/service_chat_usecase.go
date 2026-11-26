// Package chatusecase runs the chat of the dashboard: the conversations of a user, and turns in which the model
// calls the agent's tools on the user's behalf.
package chatusecase

import (
	"log/slog"
	"time"

	"github.com/nice-platform/example/agent/backend/internal/app/chat/internal/turn"
)

// Service holds what the chat needs. A nil Model turns the chat off: conversations can still be read and deleted.
type Service struct {
	Conversations Conversations
	Model         Model
	Tools         Tools
	// MaxSteps bounds the model calls of one turn.
	MaxSteps int
	Log      *slog.Logger
	// Now and NewID are replaced by tests.
	Now   func() time.Time
	NewID func() string

	locks turn.Locks
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s *Service) id() string {
	if s.NewID != nil {
		return s.NewID()
	}
	return turn.NewID()
}

func (s *Service) log() *slog.Logger {
	if s.Log != nil {
		return s.Log
	}
	return slog.New(slog.DiscardHandler)
}
