// Package sessionmemory keeps the browser sessions in memory: a restart signs
// everybody out. A deployment with several processes keeps them in a shared
// store instead (see the README).
package sessionmemory

import (
	"context"
	"sync"
	"time"

	"github.com/nice-platform/example/agent/backend/internal/domain"
)

type Store struct {
	mu       sync.Mutex
	sessions map[string]domain.Session
}

func NewStore() *Store { return &Store{sessions: map[string]domain.Session{}} }

// clone copies the context so a caller never changes a stored session.
func clone(s domain.Session) domain.Session {
	if s.Context != nil {
		c := *s.Context
		s.Context = &c
	}
	return s
}

func (m *Store) Create(_ context.Context, s domain.Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[s.ID] = clone(s)
	return nil
}

func (m *Store) Get(_ context.Context, id string) (domain.Session, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	return clone(s), ok, nil
}

func (m *Store) Update(_ context.Context, id string, change func(*domain.Session)) (domain.Session, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return domain.Session{}, false, nil
	}
	s = clone(s)
	change(&s)
	m.sessions[id] = clone(s)
	return s, true, nil
}

func (m *Store) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, id)
	return nil
}

func (m *Store) UsedBefore(_ context.Context, t time.Time) ([]domain.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.Session
	for _, s := range m.sessions {
		if s.LastUsed.Before(t) {
			out = append(out, clone(s))
		}
	}
	return out, nil
}

// Len is the number of sessions, for tests.
func (m *Store) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sessions)
}
