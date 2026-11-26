// Package chatmemory keeps conversations in memory; they are lost on restart.
package chatmemory

import (
	"cmp"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	chatdto "github.com/nice-platform/example/agent/backend/internal/app/chat/dto"
	"github.com/nice-platform/example/agent/backend/internal/domain"
)

type entry struct {
	conv     chatdto.Conversation
	messages []chatdto.Message
}

// Repository follows the same ownership rules as the PostgreSQL adapter, so the chat behaves the same without a
// database. It stores copies, so a caller cannot change what is stored through a slice it got back.
type Repository struct {
	mu    sync.Mutex
	convs map[string]*entry
}

func New() *Repository { return &Repository{convs: map[string]*entry{}} }

// owned answers the conversation of subject, or nil for an unknown one or one of another user. Callers hold mu.
func (r *Repository) owned(subject, id string) *entry {
	e := r.convs[id]
	if e == nil || e.conv.Subject != subject {
		return nil
	}
	return e
}

func (r *Repository) List(_ context.Context, subject string) ([]chatdto.Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []chatdto.Conversation{}
	for _, e := range r.convs {
		if e.conv.Subject == subject {
			out = append(out, e.conv)
		}
	}
	slices.SortFunc(out, func(a, b chatdto.Conversation) int {
		if c := b.UpdatedAt.Compare(a.UpdatedAt); c != 0 {
			return c
		}
		return cmp.Compare(a.ID, b.ID)
	})
	return out, nil
}

func (r *Repository) Get(_ context.Context, subject, id string) (chatdto.Conversation, []chatdto.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e := r.owned(subject, id)
	if e == nil {
		return chatdto.Conversation{}, nil, domain.ErrConversationNotFound
	}
	msgs := make([]chatdto.Message, len(e.messages))
	for i, m := range e.messages {
		msgs[i] = copyMessage(m)
	}
	return e.conv, msgs, nil
}

func (r *Repository) Create(_ context.Context, c chatdto.Conversation) error {
	if c.ID == "" || c.Subject == "" {
		return errors.New("a conversation needs an ID and a subject")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.convs[c.ID]; ok {
		return fmt.Errorf("conversation %s already exists", c.ID)
	}
	r.convs[c.ID] = &entry{conv: c}
	return nil
}

func (r *Repository) Rename(_ context.Context, subject, id, title string, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	e := r.owned(subject, id)
	if e == nil {
		return domain.ErrConversationNotFound
	}
	e.conv.Title, e.conv.UpdatedAt = title, at
	return nil
}

func (r *Repository) Delete(_ context.Context, subject, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.owned(subject, id) == nil {
		return domain.ErrConversationNotFound
	}
	delete(r.convs, id)
	return nil
}

func (r *Repository) Append(_ context.Context, subject, conversationID string, m chatdto.Message) (chatdto.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e := r.owned(subject, conversationID)
	if e == nil {
		return chatdto.Message{}, domain.ErrConversationNotFound
	}
	m = copyMessage(m)
	if m.ID == "" {
		m.ID = newID()
	}
	m.ConversationID = conversationID
	m.Seq = 1
	if n := len(e.messages); n > 0 {
		m.Seq = e.messages[n-1].Seq + 1
	}
	e.messages = append(e.messages, m)
	e.conv.UpdatedAt = m.CreatedAt
	return copyMessage(m), nil
}

func (r *Repository) UpdateCall(_ context.Context, subject, conversationID string, call chatdto.ToolCall) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	e := r.owned(subject, conversationID)
	if e == nil {
		return domain.ErrConversationNotFound
	}
	for i := range e.messages {
		for j, p := range e.messages[i].Parts {
			if p.Kind == domain.PartToolCall && p.Call != nil && p.Call.ID == call.ID {
				c := copyCall(call)
				e.messages[i].Parts[j].Call = &c
				return nil
			}
		}
	}
	return domain.ErrCallNotPending
}

// newID is a random UUIDv4, the same shape of ID the PostgreSQL adapter stores.
func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func copyRaw(b json.RawMessage) json.RawMessage {
	if b == nil {
		return nil
	}
	return slices.Clone(b)
}

func copyCall(c chatdto.ToolCall) chatdto.ToolCall {
	c.Input = copyRaw(c.Input)
	return c
}

func copyMessage(m chatdto.Message) chatdto.Message {
	if m.Parts == nil {
		return m
	}
	parts := make([]chatdto.Part, len(m.Parts))
	for i, p := range m.Parts {
		if p.Call != nil {
			c := copyCall(*p.Call)
			p.Call = &c
		}
		if p.Result != nil {
			res := *p.Result
			res.Content = copyRaw(res.Content)
			p.Result = &res
		}
		parts[i] = p
	}
	m.Parts = parts
	return m
}
