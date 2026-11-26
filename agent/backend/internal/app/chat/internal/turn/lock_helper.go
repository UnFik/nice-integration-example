// Package turn holds what a chat turn needs besides the use cases: the lock of a running turn, the system prompt,
// the shape of the history the model reads, and new IDs.
package turn

import "sync"

// Locks lets one turn run per conversation. A second turn is refused, not queued: the user sees that one runs.
type Locks struct {
	mu      sync.Mutex
	running map[string]struct{}
}

// TryLock takes the lock of id and returns its release, or false when a turn of id already runs.
func (l *Locks) TryLock(id string) (func(), bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.running == nil {
		l.running = map[string]struct{}{}
	}
	if _, busy := l.running[id]; busy {
		return nil, false
	}
	l.running[id] = struct{}{}
	return func() {
		l.mu.Lock()
		delete(l.running, id)
		l.mu.Unlock()
	}, true
}
