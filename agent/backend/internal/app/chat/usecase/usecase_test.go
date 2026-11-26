package chatusecase_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	calldto "github.com/nice-platform/example/agent/backend/internal/app/call/dto"
	chatdto "github.com/nice-platform/example/agent/backend/internal/app/chat/dto"
	chatusecase "github.com/nice-platform/example/agent/backend/internal/app/chat/usecase"
	"github.com/nice-platform/example/agent/backend/internal/domain"
)

// store is a small Conversations for these tests; the real ones have their own contract tests.
type store struct {
	mu    sync.Mutex
	convs map[string]chatdto.Conversation
	msgs  map[string][]chatdto.Message
}

func newStore() *store {
	return &store{convs: map[string]chatdto.Conversation{}, msgs: map[string][]chatdto.Message{}}
}

func (s *store) own(subject, id string) (chatdto.Conversation, error) {
	c, ok := s.convs[id]
	if !ok || c.Subject != subject {
		return c, domain.ErrConversationNotFound
	}
	return c, nil
}

func (s *store) List(_ context.Context, subject string) ([]chatdto.Conversation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []chatdto.Conversation
	for _, c := range s.convs {
		if c.Subject == subject {
			out = append(out, c)
		}
	}
	slices.SortFunc(out, func(a, b chatdto.Conversation) int { return b.UpdatedAt.Compare(a.UpdatedAt) })
	return out, nil
}

func (s *store) Get(_ context.Context, subject, id string) (chatdto.Conversation, []chatdto.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := s.own(subject, id)
	if err != nil {
		return c, nil, err
	}
	var out []chatdto.Message
	for _, m := range s.msgs[id] {
		m.Parts = clone(m.Parts)
		out = append(out, m)
	}
	return c, out, nil
}

func clone(p []chatdto.Part) []chatdto.Part {
	b, _ := json.Marshal(p)
	var out []chatdto.Part
	_ = json.Unmarshal(b, &out)
	return out
}

func (s *store) Create(_ context.Context, c chatdto.Conversation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.convs[c.ID] = c
	return nil
}

func (s *store) Rename(_ context.Context, subject, id, title string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := s.own(subject, id)
	if err != nil {
		return err
	}
	c.Title, c.UpdatedAt = title, at
	s.convs[id] = c
	return nil
}

func (s *store) Delete(_ context.Context, subject, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.own(subject, id); err != nil {
		return err
	}
	delete(s.convs, id)
	delete(s.msgs, id)
	return nil
}

func (s *store) Append(_ context.Context, subject, id string, m chatdto.Message) (chatdto.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := s.own(subject, id)
	if err != nil {
		return m, err
	}
	m.ConversationID, m.Seq, m.Parts = id, len(s.msgs[id])+1, clone(m.Parts)
	s.msgs[id] = append(s.msgs[id], m)
	c.UpdatedAt = m.CreatedAt
	s.convs[id] = c
	return m, nil
}

func (s *store) UpdateCall(_ context.Context, subject, id string, call chatdto.ToolCall) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.own(subject, id); err != nil {
		return err
	}
	for i, m := range s.msgs[id] {
		for j, p := range m.Parts {
			if p.Call != nil && p.Call.ID == call.ID {
				c := call
				s.msgs[id][i].Parts[j].Call = &c
				return nil
			}
		}
	}
	return domain.ErrCallNotPending
}

// model answers with its replies in order; each request is kept.
type model struct {
	mu       sync.Mutex
	replies  []chatdto.ModelReply
	always   *chatdto.ModelReply
	err      error
	block    chan struct{}
	requests []chatdto.ModelRequest
}

func (m *model) Next(ctx context.Context, req chatdto.ModelRequest) (chatdto.ModelReply, error) {
	if m.block != nil {
		<-m.block
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests = append(m.requests, req)
	if m.err != nil {
		return chatdto.ModelReply{}, m.err
	}
	if m.always != nil {
		return *m.always, nil
	}
	if len(m.replies) == 0 {
		return chatdto.ModelReply{Text: "selesai"}, nil
	}
	r := m.replies[0]
	m.replies = m.replies[1:]
	return r, nil
}

// tools fakes the agent's tools. deleteDevice is sensitive: it needs the state "ok-<call id>" and an acceptance,
// "expired" asks again.
type tools struct {
	mu    sync.Mutex
	sent  []string
	fails map[string]string
}

func (t *tools) Specs() []chatdto.ToolSpec {
	return []chatdto.ToolSpec{{Name: "list_operations"}, {Name: "call_operation"}}
}

func (t *tools) Run(_ context.Context, c calldto.Caller, call chatdto.ToolCall, ap *calldto.Approval) chatdto.ToolOutcome {
	t.mu.Lock()
	defer t.mu.Unlock()
	if code, ok := t.fails[call.Name]; ok {
		return chatdto.ToolOutcome{IsError: true, Content: json.RawMessage(`{"error":"` + code + `","message":"you may not call nadi"}`)}
	}
	var in struct{ Operation string }
	_ = json.Unmarshal(call.Input, &in)
	if in.Operation == "deleteDevice" {
		switch {
		case ap == nil || ap.State == "expired":
			return chatdto.ToolOutcome{Approval: &calldto.ApprovalRequest{Message: "Approve Delete a device", State: "ok-" + call.ID}}
		case ap.State != "ok-"+call.ID:
			return chatdto.ToolOutcome{Approval: &calldto.ApprovalRequest{Message: "Approve again", State: "ok-" + call.ID}}
		case !ap.Accepted:
			return chatdto.ToolOutcome{IsError: true, Content: json.RawMessage(`{"error":"not_approved","message":"the user did not approve"}`)}
		}
		t.sent = append(t.sent, c.Subject+":DELETE")
		return chatdto.ToolOutcome{Content: json.RawMessage(`{"status":204}`)}
	}
	t.sent = append(t.sent, c.Subject+":"+call.Name)
	return chatdto.ToolOutcome{Content: json.RawMessage(`{"status":200,"body":{"devices":[{"id":"dev_1","name":"Sensor"}]}}`)}
}

type env struct {
	svc    *chatusecase.Service
	store  *store
	model  *model
	tools  *tools
	caller calldto.Caller
	conv   chatdto.Conversation
}

func setup(t *testing.T, replies ...chatdto.ModelReply) *env {
	t.Helper()
	e := &env{store: newStore(), model: &model{replies: replies}, tools: &tools{},
		caller: calldto.Caller{Token: "secret-user-token-7f3a", Subject: "usr_a", OrgID: "org_1", ProjectID: "prj_1"}}
	n := 0
	e.svc = &chatusecase.Service{Conversations: e.store, Model: e.model, Tools: e.tools, MaxSteps: 4,
		NewID: func() string { n++; return fmt.Sprintf("id-%03d", n) }}
	conv, err := e.svc.CreateConversation(context.Background(), e.caller, "")
	if err != nil {
		t.Fatal(err)
	}
	e.conv = conv
	return e
}

func call(id, name, input string) chatdto.ToolCall {
	return chatdto.ToolCall{ID: id, Name: name, Input: json.RawMessage(input)}
}

type recorder struct{ events []chatdto.Event }

func (r *recorder) emit(e chatdto.Event) { r.events = append(r.events, e) }

func (r *recorder) types() string {
	var s []string
	for _, e := range r.events {
		t := e.Type
		if e.Code != "" {
			t += "(" + e.Code + ")"
		}
		s = append(s, t)
	}
	return strings.Join(s, " ")
}

func (e *env) messages(t *testing.T) []chatdto.Message {
	t.Helper()
	_, msgs, err := e.store.Get(context.Background(), e.caller.Subject, e.conv.ID)
	if err != nil {
		t.Fatal(err)
	}
	return msgs
}

func TestTurnListsDevicesThroughTheTools(t *testing.T) {
	e := setup(t,
		chatdto.ModelReply{Text: "Saya cek dulu.", Calls: []chatdto.ToolCall{
			call("c1", "list_operations", `{"service":"nadi"}`),
			call("c2", "call_operation", `{"service":"nadi","operation":"listDevices"}`)}},
		chatdto.ModelReply{Text: "Ada 1 perangkat: Sensor."})
	var r recorder
	if err := e.svc.RunTurn(context.Background(), e.caller, e.conv.ID, "daftar perangkat saya", r.emit); err != nil {
		t.Fatal(err)
	}
	if got := r.types(); got != "assistant tool_call tool_result tool_call tool_result assistant done(answered)" {
		t.Fatalf("events: %s", got)
	}
	var roles []string
	for _, m := range e.messages(t) {
		roles = append(roles, m.Role)
	}
	if strings.Join(roles, ",") != "user,assistant,tool,tool,assistant" {
		t.Fatalf("stored: %v", roles)
	}
	second := e.model.requests[1]
	last := second.Messages[len(second.Messages)-1]
	if last.Role != domain.RoleTool || len(last.Parts) != 2 {
		t.Fatalf("the results of one reply reach the model together: %+v", last)
	}
	if !strings.Contains(second.System, "org_1") || strings.Contains(second.System, e.caller.Token) {
		t.Fatalf("system prompt: %s", second.System)
	}
	conv, _, _ := e.store.Get(context.Background(), "usr_a", e.conv.ID)
	if conv.Title != "daftar perangkat saya" {
		t.Fatalf("title: %q", conv.Title)
	}
	for _, m := range e.messages(t) {
		b, _ := json.Marshal(m.Parts)
		if strings.Contains(string(b), e.caller.Token) || strings.Contains(string(b), "eyJ") {
			t.Fatalf("a token is stored: %s", b)
		}
	}
}

func TestTurnStopsAtTheStepLimit(t *testing.T) {
	e := setup(t)
	e.model.always = &chatdto.ModelReply{Calls: []chatdto.ToolCall{call("", "list_operations", `{"service":"nadi"}`)}}
	var r recorder
	if err := e.svc.RunTurn(context.Background(), e.caller, e.conv.ID, "ulang terus", r.emit); err != nil {
		t.Fatal(err)
	}
	if len(e.model.requests) != 4 {
		t.Fatalf("model calls: %d", len(e.model.requests))
	}
	if !strings.HasSuffix(r.types(), "assistant done(step_limit)") {
		t.Fatalf("events: %s", r.types())
	}
	msgs := e.messages(t)
	if !strings.Contains(msgs[len(msgs)-1].Parts[0].Text, "Batas 4 langkah") {
		t.Fatalf("last message: %+v", msgs[len(msgs)-1])
	}
}

func TestToolErrorGoesToTheModelAndTheConversation(t *testing.T) {
	e := setup(t, chatdto.ModelReply{Calls: []chatdto.ToolCall{call("c1", "list_operations", `{"service":"pengiriman"}`)}},
		chatdto.ModelReply{Text: "Anda tidak boleh memanggil pengiriman."})
	e.tools.fails = map[string]string{"list_operations": "service_not_allowed"}
	var r recorder
	if err := e.svc.RunTurn(context.Background(), e.caller, e.conv.ID, "operasi pengiriman?", r.emit); err != nil {
		t.Fatal(err)
	}
	res := r.events[2]
	if res.Type != "tool_result" || !res.Message.Parts[0].Result.IsError || !strings.Contains(string(res.Message.Parts[0].Result.Content), "service_not_allowed") {
		t.Fatalf("result: %+v", res)
	}
	if r.types() != "assistant tool_call tool_result assistant done(answered)" {
		t.Fatalf("events: %s", r.types())
	}
}

func TestModelErrorEndsTheTurnWithAnErrorEvent(t *testing.T) {
	e := setup(t)
	e.model.err = fmt.Errorf("anthropic: status 429: %w", domain.ErrModelUnavailable)
	var r recorder
	if err := e.svc.RunTurn(context.Background(), e.caller, e.conv.ID, "halo", r.emit); err != nil {
		t.Fatal(err)
	}
	if r.types() != "error(model_unavailable)" {
		t.Fatalf("events: %s", r.types())
	}
}

func TestOneTurnAtATimePerConversation(t *testing.T) {
	e := setup(t)
	e.model.block = make(chan struct{})
	done := make(chan error)
	go func() {
		done <- e.svc.RunTurn(context.Background(), e.caller, e.conv.ID, "pertama", func(chatdto.Event) {})
	}()
	time.Sleep(50 * time.Millisecond)
	if err := e.svc.RunTurn(context.Background(), e.caller, e.conv.ID, "kedua", func(chatdto.Event) {}); !errors.Is(err, domain.ErrTurnInProgress) {
		t.Fatalf("second turn: %v", err)
	}
	if err := e.svc.AnswerApproval(context.Background(), e.caller, e.conv.ID, "x", true, func(chatdto.Event) {}); !errors.Is(err, domain.ErrTurnInProgress) {
		t.Fatalf("approval during a turn: %v", err)
	}
	close(e.model.block)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestChatOffAndBadInputAreRefusedBeforeAnything(t *testing.T) {
	e := setup(t)
	var r recorder
	if err := e.svc.RunTurn(context.Background(), e.caller, e.conv.ID, strings.Repeat("a", domain.MaxMessageRunes+1), r.emit); !errors.As(err, new(domain.TooLongError)) {
		t.Fatalf("long: %v", err)
	}
	if err := e.svc.RunTurn(context.Background(), calldto.Caller{Subject: "usr_b"}, e.conv.ID, "halo", r.emit); !errors.Is(err, domain.ErrConversationNotFound) {
		t.Fatalf("other user: %v", err)
	}
	e.svc.Model = nil
	if err := e.svc.RunTurn(context.Background(), e.caller, e.conv.ID, "halo", r.emit); !errors.Is(err, domain.ErrChatOff) {
		t.Fatalf("chat off: %v", err)
	}
	if len(r.events) != 0 || len(e.messages(t)) != 0 {
		t.Fatalf("something ran: %v %v", r.types(), e.messages(t))
	}
}

func startDelete(t *testing.T) (*env, *recorder) {
	t.Helper()
	e := setup(t, chatdto.ModelReply{Calls: []chatdto.ToolCall{call("del", "call_operation",
		`{"service":"nadi","operation":"deleteDevice","path_params":{"id":"dev_1"}}`)}},
		chatdto.ModelReply{Text: "Perangkat dihapus."})
	var r recorder
	if err := e.svc.RunTurn(context.Background(), e.caller, e.conv.ID, "hapus dev_1", r.emit); err != nil {
		t.Fatal(err)
	}
	if r.types() != "assistant tool_call approval done(awaiting_approval)" || r.events[2].Text != "Approve Delete a device" || r.events[2].CallID != "del" {
		t.Fatalf("events: %s %+v", r.types(), r.events)
	}
	if len(e.tools.sent) != 0 {
		t.Fatalf("sent before approval: %v", e.tools.sent)
	}
	return e, &r
}

func TestApprovedDeleteIsSentOnceAndTheTurnGoesOn(t *testing.T) {
	e, _ := startDelete(t)
	var r recorder
	if err := e.svc.AnswerApproval(context.Background(), e.caller, e.conv.ID, "del", true, r.emit); err != nil {
		t.Fatal(err)
	}
	if r.types() != "tool_result assistant done(answered)" || !slices.Equal(e.tools.sent, []string{"usr_a:DELETE"}) {
		t.Fatalf("events %s, sent %v", r.types(), e.tools.sent)
	}
	if err := e.svc.AnswerApproval(context.Background(), e.caller, e.conv.ID, "del", true, r.emit); !errors.Is(err, domain.ErrCallNotPending) {
		t.Fatalf("answered twice: %v", err)
	}
	if len(e.tools.sent) != 1 {
		t.Fatalf("sent: %v", e.tools.sent)
	}
}

func TestDeclinedDeleteSendsNothing(t *testing.T) {
	e, _ := startDelete(t)
	var r recorder
	if err := e.svc.AnswerApproval(context.Background(), e.caller, e.conv.ID, "del", false, r.emit); err != nil {
		t.Fatal(err)
	}
	if len(e.tools.sent) != 0 || !strings.Contains(string(r.events[0].Message.Parts[0].Result.Content), "not_approved") {
		t.Fatalf("sent %v, events %+v", e.tools.sent, r.events)
	}
}

func TestExpiredApprovalAsksAgain(t *testing.T) {
	e, _ := startDelete(t)
	if err := e.store.UpdateCall(context.Background(), "usr_a", e.conv.ID, chatdto.ToolCall{ID: "del", Name: "call_operation",
		Input:  json.RawMessage(`{"service":"nadi","operation":"deleteDevice","path_params":{"id":"dev_1"}}`),
		Status: domain.CallAwaitingApproval, ApprovalState: "expired", ApprovalMessage: "old"}); err != nil {
		t.Fatal(err)
	}
	var r recorder
	if err := e.svc.AnswerApproval(context.Background(), e.caller, e.conv.ID, "del", true, r.emit); err != nil {
		t.Fatal(err)
	}
	if r.types() != "approval done(awaiting_approval)" || len(e.tools.sent) != 0 {
		t.Fatalf("events %s, sent %v", r.types(), e.tools.sent)
	}
}

func TestAnotherUserCannotAnswer(t *testing.T) {
	e, _ := startDelete(t)
	other := calldto.Caller{Subject: "usr_b", OrgID: "org_1", ProjectID: "prj_1"}
	if err := e.svc.AnswerApproval(context.Background(), other, e.conv.ID, "del", true, func(chatdto.Event) {}); !errors.Is(err, domain.ErrConversationNotFound) {
		t.Fatalf("other user: %v", err)
	}
	var waiting bool
	for _, m := range e.messages(t) {
		for _, p := range m.Parts {
			waiting = waiting || (p.Call != nil && p.Call.Status == domain.CallAwaitingApproval)
		}
	}
	if !waiting || len(e.tools.sent) != 0 {
		t.Fatalf("the call no longer waits or was sent: %v", e.tools.sent)
	}
}

func TestANewMessageDeclinesTheWaitingCall(t *testing.T) {
	e, _ := startDelete(t)
	e.model.replies = []chatdto.ModelReply{{Text: "Baik."}}
	var r recorder
	if err := e.svc.RunTurn(context.Background(), e.caller, e.conv.ID, "batal saja", r.emit); err != nil {
		t.Fatal(err)
	}
	if r.types() != "tool_result assistant done(answered)" || len(e.tools.sent) != 0 {
		t.Fatalf("events %s, sent %v", r.types(), e.tools.sent)
	}
	last := e.model.requests[len(e.model.requests)-1].Messages
	if last[len(last)-2].Role != domain.RoleTool || last[len(last)-1].Role != domain.RoleUser {
		t.Fatalf("the model must see the declined result before the new message: %+v", last)
	}
}

func TestConversationsBelongToTheirUser(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	if err := e.svc.RenameConversation(ctx, e.caller, e.conv.ID, "  Perangkat  "); err != nil {
		t.Fatal(err)
	}
	other := calldto.Caller{Subject: "usr_b"}
	if err := e.svc.RenameConversation(ctx, other, e.conv.ID, "x"); !errors.Is(err, domain.ErrConversationNotFound) {
		t.Fatalf("rename by another user: %v", err)
	}
	if err := e.svc.RenameConversation(ctx, e.caller, e.conv.ID, " "); err == nil {
		t.Fatal("empty title accepted")
	}
	if list, _ := e.svc.ListConversations(ctx, other); len(list) != 0 {
		t.Fatalf("another user sees %v", list)
	}
	list, _ := e.svc.ListConversations(ctx, e.caller)
	if len(list) != 1 || list[0].Title != "Perangkat" {
		t.Fatalf("list: %+v", list)
	}
	if err := e.svc.DeleteConversation(ctx, other, e.conv.ID); !errors.Is(err, domain.ErrConversationNotFound) {
		t.Fatalf("delete by another user: %v", err)
	}
	if err := e.svc.DeleteConversation(ctx, e.caller, e.conv.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := e.svc.GetConversation(ctx, e.caller, e.conv.ID); !errors.Is(err, domain.ErrConversationNotFound) {
		t.Fatalf("after delete: %v", err)
	}
}
