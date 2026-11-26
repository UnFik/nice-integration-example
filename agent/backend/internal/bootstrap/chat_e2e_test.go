package bootstrap

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	chatmemory "github.com/nice-platform/example/agent/backend/internal/adapter/memory/chat"
	calldto "github.com/nice-platform/example/agent/backend/internal/app/call/dto"
	chatdto "github.com/nice-platform/example/agent/backend/internal/app/chat/dto"
	chatusecase "github.com/nice-platform/example/agent/backend/internal/app/chat/usecase"
	"github.com/nice-platform/example/agent/backend/internal/platform/config"
)

type scriptedModel struct{ replies []chatdto.ModelReply }

func (m *scriptedModel) Next(context.Context, chatdto.ModelRequest) (chatdto.ModelReply, error) {
	if len(m.replies) == 0 {
		return chatdto.ModelReply{Text: "Selesai."}, nil
	}
	r := m.replies[0]
	m.replies = m.replies[1:]
	return r, nil
}

// recordingTools answers every call and keeps the callers it ran for.
type recordingTools struct{ callers []calldto.Caller }

func (*recordingTools) Specs() []chatdto.ToolSpec { return []chatdto.ToolSpec{{Name: "list_services"}} }

func (r *recordingTools) Run(_ context.Context, c calldto.Caller, _ chatdto.ToolCall, _ *calldto.Approval) chatdto.ToolOutcome {
	r.callers = append(r.callers, c)
	return chatdto.ToolOutcome{Content: json.RawMessage(`{"organizations":[]}`)}
}

// The chat behind the session middleware: the turn runs for the signed-in user, in the active context.
func TestDashboardTurnRunsForTheSignedInUser(t *testing.T) {
	tools := &recordingTools{}
	svc := &chatusecase.Service{Conversations: chatmemory.New(), Tools: tools, MaxSteps: 4, Model: &scriptedModel{replies: []chatdto.ModelReply{
		{Calls: []chatdto.ToolCall{{ID: "c1", Name: "list_services", Input: json.RawMessage(`{}`)}}},
		{Text: "Anda belum punya service."},
	}}}
	e := newBFFEnv(t, NewChatHandler(config.Config{LLM: config.LLM{Provider: config.ProviderAnthropic}}, svc, nil))
	b := e.browser()
	b.login()

	r := b.do(http.MethodPost, "/chat/conversations", map[string]any{})
	if r.Status != http.StatusCreated {
		t.Fatalf("create = %d %s", r.Status, r.Body)
	}
	id := r.JSON(t)["conversation"].(map[string]any)["id"].(string)
	r = b.do(http.MethodPost, "/chat/conversations/"+id+"/messages", map[string]any{"text": "service apa saja?"})
	if r.Status != http.StatusOK || !strings.Contains(r.Body, "event: done") || !strings.Contains(r.Body, "Anda belum punya service.") {
		t.Fatalf("turn = %d %s", r.Status, r.Body)
	}
	if strings.Contains(r.Body, "eyJ") {
		t.Fatalf("a token reached the browser: %s", r.Body)
	}
	if len(tools.callers) != 1 || tools.callers[0].Subject != bffUser || tools.callers[0].ProjectID != prjP || !strings.HasPrefix(tools.callers[0].Token, "eyJ") {
		t.Fatalf("tool callers = %+v", tools.callers)
	}
	if r := b.do(http.MethodGet, "/chat/conversations", nil); !strings.Contains(r.Body, `"chat_enabled":true`) || !strings.Contains(r.Body, id) {
		t.Fatalf("list = %s", r.Body)
	}
	// The same request with a Bearer token instead of the session is refused before the chat.
	if r := e.do(http.MethodGet, "/chat/conversations", e.nice.token(map[string]any{"sub": bffUser}), nil); r.Status != http.StatusUnauthorized {
		t.Fatalf("bearer = %d", r.Status)
	}
}
