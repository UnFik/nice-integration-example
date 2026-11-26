// Package chattest holds the contract tests every chatusecase.Conversations adapter must pass, so the memory and
// PostgreSQL stores behave the same.
package chattest

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	chatdto "github.com/nice-platform/example/agent/backend/internal/app/chat/dto"
	chatusecase "github.com/nice-platform/example/agent/backend/internal/app/chat/usecase"
	"github.com/nice-platform/example/agent/backend/internal/domain"
)

// Base is the clock of the suite, at microsecond precision because PostgreSQL keeps no more.
var Base = time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)

// NewID is a random UUIDv4 in canonical form.
func NewID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// NewConversation creates a conversation of subject last changed at at.
func NewConversation(t *testing.T, repo chatusecase.Conversations, subject, title string, at time.Time) chatdto.Conversation {
	t.Helper()
	c := chatdto.Conversation{ID: NewID(), Subject: subject, Title: title, CreatedAt: at, UpdatedAt: at}
	if err := repo.Create(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	return c
}

// Text is a message of role holding one text part.
func Text(role, text string, at time.Time) chatdto.Message {
	return chatdto.Message{Role: role, Parts: []chatdto.Part{{Kind: domain.PartText, Text: text}}, CreatedAt: at}
}

// Call is an assistant message asking for the calls with ids, each waiting for approval.
func Call(at time.Time, ids ...string) chatdto.Message {
	m := chatdto.Message{Role: domain.RoleAssistant, CreatedAt: at, Parts: []chatdto.Part{{Kind: domain.PartText, Text: "Let me check."}}}
	for _, id := range ids {
		m.Parts = append(m.Parts, chatdto.Part{Kind: domain.PartToolCall, Call: &chatdto.ToolCall{
			ID: id, Name: "call_operation", Status: domain.CallAwaitingApproval,
			Input:           json.RawMessage(`{"service":"nadi","operation":"DELETE /devices/{id}","path":{"id":"dev_1"}}`),
			ApprovalState:   "state-" + id,
			ApprovalMessage: "Delete device dev_1?",
		}})
	}
	return m
}

// Result is a tool message answering callID with content.
func Result(callID string, content string, at time.Time) chatdto.Message {
	return chatdto.Message{Role: domain.RoleTool, CreatedAt: at, Parts: []chatdto.Part{{Kind: domain.PartToolResult,
		Result: &chatdto.ToolResult{CallID: callID, Content: json.RawMessage(content)}}}}
}

// SameJSON compares the JSON of got and want, so a store may reformat raw JSON (jsonb does).
func SameJSON(t *testing.T, what string, got, want any) {
	t.Helper()
	norm := func(v any) any {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		var out any
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	if g, w := norm(got), norm(want); !reflect.DeepEqual(g, w) {
		t.Errorf("%s = %v, want %v", what, g, w)
	}
}

func ids(cs []chatdto.Conversation) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.ID
	}
	return out
}

// Run runs the contract tests. open gives an empty store for each subtest.
func Run(t *testing.T, open func(t *testing.T) chatusecase.Conversations) {
	ctx := context.Background()

	t.Run("ListIsPerUserLatestFirst", func(t *testing.T) {
		repo := open(t)
		old := NewConversation(t, repo, "usr_alice", "old", Base)
		newest := NewConversation(t, repo, "usr_alice", "newest", Base.Add(2*time.Hour))
		mid := NewConversation(t, repo, "usr_alice", "mid", Base.Add(time.Hour))
		// Same UpdatedAt: ordered by ID.
		tieA := chatdto.Conversation{ID: "00000000-0000-4000-8000-000000000001", Subject: "usr_alice", Title: "tie a", CreatedAt: Base.Add(-time.Hour), UpdatedAt: Base.Add(-time.Hour)}
		tieB := tieA
		tieB.ID, tieB.Title = "00000000-0000-4000-8000-000000000002", "tie b"
		for _, c := range []chatdto.Conversation{tieB, tieA} {
			if err := repo.Create(ctx, c); err != nil {
				t.Fatal(err)
			}
		}
		bobs := NewConversation(t, repo, "usr_bob", "bob", Base.Add(3*time.Hour))

		got, err := repo.List(ctx, "usr_alice")
		if err != nil {
			t.Fatal(err)
		}
		want := []string{newest.ID, mid.ID, old.ID, tieA.ID, tieB.ID}
		if !reflect.DeepEqual(ids(got), want) {
			t.Fatalf("alice's list = %v, want %v", ids(got), want)
		}
		if got[0] != newest {
			t.Errorf("first = %+v, want %+v", got[0], newest)
		}
		if got, _ := repo.List(ctx, "usr_bob"); !reflect.DeepEqual(ids(got), []string{bobs.ID}) {
			t.Errorf("bob's list = %v, want only %s", ids(got), bobs.ID)
		}
		if got, err := repo.List(ctx, "usr_carol"); err != nil || len(got) != 0 {
			t.Errorf("carol's list = %v, %v; want empty", got, err)
		}
	})

	t.Run("AnotherUsersConversationIsNotFound", func(t *testing.T) {
		repo := open(t)
		c := NewConversation(t, repo, "usr_alice", "mine", Base)
		if _, err := repo.Append(ctx, "usr_alice", c.ID, Call(Base.Add(time.Minute), "call_1")); err != nil {
			t.Fatal(err)
		}
		for _, who := range []struct{ subject, id string }{
			{"usr_bob", c.ID},
			{"usr_alice", NewID()},
			{"usr_alice", "not-a-uuid"},
			{"", c.ID},
		} {
			check := func(op string, err error) {
				t.Helper()
				if !errors.Is(err, domain.ErrConversationNotFound) {
					t.Errorf("%s by %q of %q: err = %v, want ErrConversationNotFound", op, who.subject, who.id, err)
				}
			}
			_, _, err := repo.Get(ctx, who.subject, who.id)
			check("Get", err)
			check("Rename", repo.Rename(ctx, who.subject, who.id, "theirs", Base.Add(time.Hour)))
			_, err = repo.Append(ctx, who.subject, who.id, Text(domain.RoleUser, "hi", Base.Add(time.Hour)))
			check("Append", err)
			check("UpdateCall", repo.UpdateCall(ctx, who.subject, who.id, chatdto.ToolCall{ID: "call_1", Status: domain.CallDone}))
			check("Delete", repo.Delete(ctx, who.subject, who.id))
		}
		got, msgs, err := repo.Get(ctx, "usr_alice", c.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Title != "mine" || !got.UpdatedAt.Equal(Base.Add(time.Minute)) || len(msgs) != 1 ||
			msgs[0].Parts[1].Call.Status != domain.CallAwaitingApproval {
			t.Errorf("after the other calls: %+v %+v, want it unchanged", got, msgs)
		}
	})

	t.Run("RenameSetsTitleAndUpdatedAt", func(t *testing.T) {
		repo := open(t)
		c := NewConversation(t, repo, "usr_alice", "first", Base)
		if err := repo.Rename(ctx, "usr_alice", c.ID, "second", Base.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		got, _, err := repo.Get(ctx, "usr_alice", c.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Title != "second" || !got.UpdatedAt.Equal(Base.Add(time.Hour)) || !got.CreatedAt.Equal(Base) {
			t.Errorf("renamed = %+v", got)
		}
	})

	t.Run("DeleteRemovesMessages", func(t *testing.T) {
		repo := open(t)
		c := NewConversation(t, repo, "usr_alice", "gone", Base)
		keep := NewConversation(t, repo, "usr_alice", "kept", Base)
		for _, id := range []string{c.ID, keep.ID} {
			if _, err := repo.Append(ctx, "usr_alice", id, Text(domain.RoleUser, "hi", Base.Add(time.Minute))); err != nil {
				t.Fatal(err)
			}
		}
		if err := repo.Delete(ctx, "usr_alice", c.ID); err != nil {
			t.Fatal(err)
		}
		if _, _, err := repo.Get(ctx, "usr_alice", c.ID); !errors.Is(err, domain.ErrConversationNotFound) {
			t.Errorf("Get after Delete: err = %v", err)
		}
		if err := repo.Delete(ctx, "usr_alice", c.ID); !errors.Is(err, domain.ErrConversationNotFound) {
			t.Errorf("second Delete: err = %v", err)
		}
		if got, _ := repo.List(ctx, "usr_alice"); !reflect.DeepEqual(ids(got), []string{keep.ID}) {
			t.Errorf("list after Delete = %v, want only %s", ids(got), keep.ID)
		}
		if _, msgs, err := repo.Get(ctx, "usr_alice", keep.ID); err != nil || len(msgs) != 1 {
			t.Errorf("the other conversation: %v, %v", msgs, err)
		}
	})

	t.Run("AppendNumbersMessagesAndMovesUpdatedAt", func(t *testing.T) {
		repo := open(t)
		c := NewConversation(t, repo, "usr_alice", "chat", Base)
		other := NewConversation(t, repo, "usr_alice", "other", Base.Add(time.Minute))
		sent := []chatdto.Message{
			Text(domain.RoleUser, "list my devices", Base.Add(1*time.Second)),
			Call(Base.Add(2*time.Second), "call_1"),
			Result("call_1", `{"status": 200, "body": {"devices": [{"id": "dev_1"}]}}`, Base.Add(3*time.Second)),
			Text(domain.RoleAssistant, "You have one device.", Base.Add(4*time.Second)),
		}
		for i, m := range sent {
			got, err := repo.Append(ctx, "usr_alice", c.ID, m)
			if err != nil {
				t.Fatal(err)
			}
			if got.Seq != i+1 || got.ID == "" || got.ConversationID != c.ID {
				t.Errorf("append %d = seq %d id %q conversation %q", i, got.Seq, got.ID, got.ConversationID)
			}
			sent[i].ID = got.ID
		}
		conv, msgs, err := repo.Get(ctx, "usr_alice", c.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !conv.UpdatedAt.Equal(Base.Add(4 * time.Second)) {
			t.Errorf("UpdatedAt = %v, want the CreatedAt of the last message", conv.UpdatedAt)
		}
		if len(msgs) != len(sent) {
			t.Fatalf("got %d messages, want %d", len(msgs), len(sent))
		}
		for i, m := range msgs {
			if m.Seq != i+1 || m.ID != sent[i].ID || m.Role != sent[i].Role || !m.CreatedAt.Equal(sent[i].CreatedAt) || m.ConversationID != c.ID {
				t.Errorf("message %d = %+v", i, m)
			}
			SameJSON(t, fmt.Sprintf("parts of message %d", i), m.Parts, sent[i].Parts)
		}
		if list, _ := repo.List(ctx, "usr_alice"); !reflect.DeepEqual(ids(list), []string{other.ID, c.ID}) {
			t.Errorf("list = %v: the appended conversation moved to %v", ids(list), Base.Add(4*time.Second))
		}
		if got, err := repo.Append(ctx, "usr_alice", other.ID, Text(domain.RoleUser, "x", Base.Add(time.Hour))); err != nil || got.Seq != 1 {
			t.Errorf("first message of another conversation: seq %d, %v", got.Seq, err)
		}
		if list, _ := repo.List(ctx, "usr_alice"); !reflect.DeepEqual(ids(list), []string{other.ID, c.ID}) {
			t.Errorf("list = %v", ids(list))
		}
	})

	t.Run("ConcurrentAppendsTakeDistinctSeq", func(t *testing.T) {
		repo := open(t)
		c := NewConversation(t, repo, "usr_alice", "busy", Base)
		const n = 8
		var wg sync.WaitGroup
		errs := make(chan error, n)
		for i := range n {
			wg.Go(func() {
				_, err := repo.Append(ctx, "usr_alice", c.ID, Text(domain.RoleUser, fmt.Sprint(i), Base.Add(time.Duration(i)*time.Second)))
				errs <- err
			})
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatal(err)
			}
		}
		_, msgs, err := repo.Get(ctx, "usr_alice", c.ID)
		if err != nil {
			t.Fatal(err)
		}
		for i, m := range msgs {
			if m.Seq != i+1 {
				t.Errorf("message %d has seq %d", i, m.Seq)
			}
		}
		if len(msgs) != n {
			t.Errorf("got %d messages, want %d", len(msgs), n)
		}
	})

	t.Run("UpdateCallReplacesThatPartOnly", func(t *testing.T) {
		repo := open(t)
		c := NewConversation(t, repo, "usr_alice", "calls", Base)
		if _, err := repo.Append(ctx, "usr_alice", c.ID, Text(domain.RoleUser, "delete dev_1", Base.Add(time.Second))); err != nil {
			t.Fatal(err)
		}
		asked, err := repo.Append(ctx, "usr_alice", c.ID, Call(Base.Add(2*time.Second), "call_a", "call_b"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := repo.Append(ctx, "usr_alice", c.ID, Result("call_r", `{"ok": true}`, Base.Add(3*time.Second))); err != nil {
			t.Fatal(err)
		}
		done := *asked.Parts[2].Call
		done.Status, done.ApprovalState, done.ApprovalMessage = domain.CallDone, "", ""
		if err := repo.UpdateCall(ctx, "usr_alice", c.ID, done); err != nil {
			t.Fatal(err)
		}
		conv, msgs, err := repo.Get(ctx, "usr_alice", c.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(msgs) != 3 || msgs[1].Seq != 2 || msgs[1].ID != asked.ID {
			t.Fatalf("messages = %+v", msgs)
		}
		want := asked.Parts
		want[2] = chatdto.Part{Kind: domain.PartToolCall, Call: &done}
		SameJSON(t, "parts after UpdateCall", msgs[1].Parts, want)
		if !conv.UpdatedAt.Equal(Base.Add(3 * time.Second)) {
			t.Errorf("UpdatedAt = %v, UpdateCall must not move it", conv.UpdatedAt)
		}

		for _, id := range []string{"call_missing", "call_r", ""} {
			if err := repo.UpdateCall(ctx, "usr_alice", c.ID, chatdto.ToolCall{ID: id, Status: domain.CallDone}); !errors.Is(err, domain.ErrCallNotPending) {
				t.Errorf("UpdateCall of %q: err = %v, want ErrCallNotPending", id, err)
			}
		}
	})

	t.Run("ReturnedDataIsACopy", func(t *testing.T) {
		repo := open(t)
		c := NewConversation(t, repo, "usr_alice", "copy", Base)
		m := Call(Base.Add(time.Second), "call_1")
		got, err := repo.Append(ctx, "usr_alice", c.ID, m)
		if err != nil {
			t.Fatal(err)
		}
		m.Parts[1].Call.Input[2] = 'X'
		m.Parts[0].Text = "changed"
		got.Parts[1].Call.Status = "changed"
		_, msgs, _ := repo.Get(ctx, "usr_alice", c.ID)
		msgs[0].Parts[1].Call.Input = json.RawMessage(`{}`)
		msgs[0].Parts[0].Text = "changed"
		_, again, err := repo.Get(ctx, "usr_alice", c.ID)
		if err != nil {
			t.Fatal(err)
		}
		SameJSON(t, "stored parts", again[0].Parts, Call(Base.Add(time.Second), "call_1").Parts)
	})
}
