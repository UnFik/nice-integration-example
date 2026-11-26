package chat_test

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	chatmemory "github.com/nice-platform/example/agent/backend/internal/adapter/memory/chat"
	"github.com/nice-platform/example/agent/backend/internal/adapter/rest/chat"
	calldto "github.com/nice-platform/example/agent/backend/internal/app/call/dto"
	chatdto "github.com/nice-platform/example/agent/backend/internal/app/chat/dto"
	chatusecase "github.com/nice-platform/example/agent/backend/internal/app/chat/usecase"
)

const userToken = "eyJhbGciOiJSUzI1NiJ9.user-token.sig"

type model struct {
	mu      sync.Mutex
	replies []chatdto.ModelReply
	block   chan struct{}
	entered chan struct{}
}

func (m *model) Next(context.Context, chatdto.ModelRequest) (chatdto.ModelReply, error) {
	if m.block != nil {
		select {
		case m.entered <- struct{}{}:
		default:
		}
		<-m.block
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.replies) == 0 {
		return chatdto.ModelReply{Text: "Selesai."}, nil
	}
	r := m.replies[0]
	m.replies = m.replies[1:]
	return r, nil
}

type tools struct{ deleted int }

func (*tools) Specs() []chatdto.ToolSpec { return []chatdto.ToolSpec{{Name: "call_operation"}} }

func (t *tools) Run(_ context.Context, _ calldto.Caller, call chatdto.ToolCall, ap *calldto.Approval) chatdto.ToolOutcome {
	if strings.Contains(string(call.Input), "deleteDevice") {
		if ap == nil || !ap.Accepted {
			if ap != nil {
				return chatdto.ToolOutcome{IsError: true, Content: json.RawMessage(`{"error":"not_approved"}`)}
			}
			return chatdto.ToolOutcome{Approval: &calldto.ApprovalRequest{Message: "Approve Delete a device", State: "state-1"}}
		}
		t.deleted++
		return chatdto.ToolOutcome{Content: json.RawMessage(`{"status":204}`)}
	}
	return chatdto.ToolOutcome{Content: json.RawMessage(`{"status":200,"body":{"devices":[]}}`)}
}

type server struct {
	*httptest.Server
	model *model
	tools *tools
}

// newServer serves the chat routes; the X-Test-User header stands in for the session middleware.
func newServer(t *testing.T, enabled bool) *server {
	t.Helper()
	s := &server{model: &model{}, tools: &tools{}}
	svc := &chatusecase.Service{Conversations: chatmemory.New(), Tools: s.tools, MaxSteps: 4}
	if enabled {
		svc.Model = s.model
	}
	h := &chat.Handler{Chat: svc, ChatEnabled: enabled, Caller: func(r *http.Request) (calldto.Caller, bool) {
		u := r.Header.Get("X-Test-User")
		return calldto.Caller{Token: userToken, Subject: u, OrgID: "org_1", ProjectID: "prj_1"}, u != ""
	}}
	s.Server = httptest.NewServer(h.Routes())
	t.Cleanup(s.Close)
	return s
}

func (s *server) do(t *testing.T, user, method, path, body string) (*http.Response, string) {
	t.Helper()
	req, _ := http.NewRequest(method, s.URL+path, strings.NewReader(body))
	if user != "" {
		req.Header.Set("X-Test-User", user)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if strings.Contains(string(b), userToken) {
		t.Fatalf("%s %s: the user token is in the body: %s", method, path, b)
	}
	return res, string(b)
}

func (s *server) create(t *testing.T, user string) string {
	t.Helper()
	res, body := s.do(t, user, "POST", "/chat/conversations", `{}`)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d %s", res.StatusCode, body)
	}
	var out struct{ Conversation struct{ ID string } }
	_ = json.Unmarshal([]byte(body), &out)
	return out.Conversation.ID
}

type event struct {
	Type string
	Data map[string]any
}

func events(t *testing.T, body string) []event {
	t.Helper()
	var out []event
	sc := bufio.NewScanner(strings.NewReader(body))
	var cur event
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			cur.Type = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &cur.Data); err != nil {
				t.Fatalf("data: %v", err)
			}
		case line == "":
			out = append(out, cur)
			cur = event{}
		}
	}
	return out
}

func types(es []event) string {
	var s []string
	for _, e := range es {
		s = append(s, e.Type)
	}
	return strings.Join(s, " ")
}

func TestSendStreamsTheStepsOfATurn(t *testing.T) {
	s := newServer(t, true)
	s.model.replies = []chatdto.ModelReply{
		{Calls: []chatdto.ToolCall{{ID: "c1", Name: "call_operation", Input: json.RawMessage(`{"service":"nadi","operation":"listDevices"}`)}}},
		{Text: "Belum ada perangkat."},
	}
	id := s.create(t, "usr_a")
	res, body := s.do(t, "usr_a", "POST", "/chat/conversations/"+id+"/messages", `{"text":"daftar perangkat"}`)
	if res.StatusCode != 200 || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("%d %s", res.StatusCode, res.Header.Get("Content-Type"))
	}
	es := events(t, body)
	if types(es) != "assistant tool_call tool_result assistant done" || es[4].Data["reason"] != "answered" {
		t.Fatalf("events: %s\n%s", types(es), body)
	}
	_, detail := s.do(t, "usr_a", "GET", "/chat/conversations/"+id, "")
	if !strings.Contains(detail, `"title":"daftar perangkat"`) || !strings.Contains(detail, "Belum ada perangkat.") {
		t.Fatalf("detail: %s", detail)
	}
}

func TestApprovalOverHTTP(t *testing.T) {
	s := newServer(t, true)
	s.model.replies = []chatdto.ModelReply{
		{Calls: []chatdto.ToolCall{{ID: "del", Name: "call_operation", Input: json.RawMessage(`{"operation":"deleteDevice"}`)}}},
		{Text: "Dihapus."},
	}
	id := s.create(t, "usr_a")
	_, body := s.do(t, "usr_a", "POST", "/chat/conversations/"+id+"/messages", `{"text":"hapus"}`)
	es := events(t, body)
	if types(es) != "assistant tool_call approval done" || es[2].Data["message"] != "Approve Delete a device" {
		t.Fatalf("events: %s", body)
	}
	_, detail := s.do(t, "usr_a", "GET", "/chat/conversations/"+id, "")
	if strings.Contains(detail, "state-1") || !strings.Contains(detail, `"status":"awaiting_approval"`) {
		t.Fatalf("detail must show the waiting call without its state: %s", detail)
	}
	if res, _ := s.do(t, "usr_b", "POST", "/chat/conversations/"+id+"/approvals/del", `{"approve":true}`); res.StatusCode != 404 {
		t.Fatalf("another user: %d", res.StatusCode)
	}
	if res, _ := s.do(t, "usr_a", "POST", "/chat/conversations/"+id+"/approvals/del", `{}`); res.StatusCode != 400 {
		t.Fatalf("no answer: %d", res.StatusCode)
	}
	_, body = s.do(t, "usr_a", "POST", "/chat/conversations/"+id+"/approvals/del", `{"approve":true}`)
	if types(events(t, body)) != "tool_result assistant done" || s.tools.deleted != 1 {
		t.Fatalf("approve: %s deleted=%d", body, s.tools.deleted)
	}
	if res, b := s.do(t, "usr_a", "POST", "/chat/conversations/"+id+"/approvals/del", `{"approve":true}`); res.StatusCode != 409 || !strings.Contains(b, "call_not_pending") {
		t.Fatalf("twice: %d %s", res.StatusCode, b)
	}
}

func TestSecondTurnIsRefusedWhileOneRuns(t *testing.T) {
	s := newServer(t, true)
	s.model.block, s.model.entered = make(chan struct{}), make(chan struct{}, 1)
	id := s.create(t, "usr_a")
	first := make(chan struct{})
	go func() {
		defer close(first)
		req, _ := http.NewRequest("POST", s.URL+"/chat/conversations/"+id+"/messages", strings.NewReader(`{"text":"satu"}`))
		req.Header.Set("X-Test-User", "usr_a")
		if res, err := http.DefaultClient.Do(req); err == nil {
			_, _ = io.Copy(io.Discard, res.Body)
			res.Body.Close()
		}
	}()
	// The first turn holds the lock once it reaches the model.
	<-s.model.entered
	res, b := s.do(t, "usr_a", "POST", "/chat/conversations/"+id+"/messages", `{"text":"dua"}`)
	close(s.model.block)
	<-first
	if res.StatusCode != 409 || !strings.Contains(b, "turn_in_progress") {
		t.Fatalf("second turn: %d %s", res.StatusCode, b)
	}
}

func TestErrorsBeforeATurnAreJSON(t *testing.T) {
	off := newServer(t, false)
	id := off.create(t, "usr_a")
	if res, b := off.do(t, "usr_a", "POST", "/chat/conversations/"+id+"/messages", `{"text":"halo"}`); res.StatusCode != 503 || !strings.Contains(b, "chat_not_configured") {
		t.Fatalf("chat off: %d %s", res.StatusCode, b)
	}
	if _, b := off.do(t, "usr_a", "GET", "/chat/conversations", ""); !strings.Contains(b, `"chat_enabled":false`) {
		t.Fatalf("list: %s", b)
	}

	s := newServer(t, true)
	id = s.create(t, "usr_a")
	if res, b := s.do(t, "usr_a", "POST", "/chat/conversations/"+id+"/messages", `{"text":"`+strings.Repeat("a", 8001)+`"}`); res.StatusCode != 413 || !strings.Contains(b, "message_too_long") {
		t.Fatalf("long: %d %s", res.StatusCode, b)
	}
	if res, _ := s.do(t, "usr_a", "POST", "/chat/conversations/"+id+"/messages", `not json`); res.StatusCode != 400 {
		t.Fatalf("bad body: %d", res.StatusCode)
	}
	if res, _ := s.do(t, "", "GET", "/chat/conversations", ""); res.StatusCode != 401 {
		t.Fatalf("no user: %d", res.StatusCode)
	}
	for _, r := range []struct{ method, path, body string }{
		{"GET", "/chat/conversations/" + id, ""},
		{"PATCH", "/chat/conversations/" + id, `{"title":"x"}`},
		{"DELETE", "/chat/conversations/" + id, ""},
		{"POST", "/chat/conversations/" + id + "/messages", `{"text":"halo"}`},
	} {
		if res, _ := s.do(t, "usr_b", r.method, r.path, r.body); res.StatusCode != 404 {
			t.Errorf("%s %s by another user: %d", r.method, r.path, res.StatusCode)
		}
	}
	if res, b := s.do(t, "usr_a", "PATCH", "/chat/conversations/"+id, `{"title":"Perangkat"}`); res.StatusCode != 200 || !strings.Contains(b, `"title":"Perangkat"`) {
		t.Fatalf("rename: %d %s", res.StatusCode, b)
	}
	if res, _ := s.do(t, "usr_a", "DELETE", "/chat/conversations/"+id, ""); res.StatusCode != 204 {
		t.Fatalf("delete: %d", res.StatusCode)
	}
	if _, b := s.do(t, "usr_a", "GET", "/chat/conversations", ""); !strings.Contains(b, `"conversations":[]`) {
		t.Fatalf("after delete: %s", b)
	}
}
