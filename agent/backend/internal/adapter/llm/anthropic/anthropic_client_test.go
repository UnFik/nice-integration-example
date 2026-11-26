package anthropicllm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	chatdto "github.com/nice-platform/example/agent/backend/internal/app/chat/dto"
	"github.com/nice-platform/example/agent/backend/internal/domain"
)

const testKey = "sk-ant-test-SECRET-0123456789"

type fakeAPI struct {
	srv    *httptest.Server
	mu     sync.Mutex
	bodies []map[string]any
	keys   []string
}

func newFakeAPI(t *testing.T, status int, reply string) *fakeAPI {
	t.Helper()
	f := &fakeAPI{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/messages" {
			http.NotFound(w, r)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		f.mu.Lock()
		f.bodies = append(f.bodies, body)
		f.keys = append(f.keys, r.Header.Get("X-Api-Key"))
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, reply)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeAPI) client() *Client {
	return newClient(Options{Model: "claude-test", BaseURL: f.srv.URL, APIKey: testKey, HTTPClient: f.srv.Client()}, 0)
}

func (f *fakeAPI) only(t *testing.T) map[string]any {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.bodies) != 1 {
		t.Fatalf("requests = %d, want 1", len(f.bodies))
	}
	return f.bodies[0]
}

func conversation() chatdto.ModelRequest {
	return chatdto.ModelRequest{
		System: "You are the agent.",
		Messages: []chatdto.Message{
			{Role: domain.RoleUser, Parts: []chatdto.Part{{Kind: domain.PartText, Text: "Which services can I call?"}}},
			{Role: domain.RoleAssistant, Parts: []chatdto.Part{
				{Kind: domain.PartText, Text: "Let me look."},
				{Kind: domain.PartToolCall, Call: &chatdto.ToolCall{ID: "toolu_1", Name: "list_services", Input: json.RawMessage(`{}`)}},
				{Kind: domain.PartToolCall, Call: &chatdto.ToolCall{ID: "toolu_2", Name: "list_operations", Input: json.RawMessage(`{"service":"nadi"}`)}},
			}},
			{Role: domain.RoleTool, Parts: []chatdto.Part{
				{Kind: domain.PartToolResult, Result: &chatdto.ToolResult{CallID: "toolu_1", Content: json.RawMessage(`{"organizations":[]}`)}},
				{Kind: domain.PartToolResult, Result: &chatdto.ToolResult{CallID: "toolu_2", Content: json.RawMessage(`{"error":"forbidden","message":"no"}`), IsError: true}},
			}},
			{Role: domain.RoleUser, Parts: []chatdto.Part{{Kind: domain.PartText, Text: "Then list the devices."}}},
		},
		Tools: []chatdto.ToolSpec{{
			Name: "list_operations", Description: "Lists the operations.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"service":{"type":"string","description":"Slug"}},"required":["service"],"additionalProperties":false}`),
		}},
	}
}

const toolUseReply = `{"id":"msg_1","type":"message","role":"assistant","model":"claude-test",
"content":[{"type":"text","text":"Checking."},{"type":"tool_use","id":"toolu_9","name":"call_operation","input":{"service":"nadi","operation":"list_devices"}}],
"stop_reason":"tool_use","usage":{"input_tokens":1,"output_tokens":1}}`

func TestNextSendsSystemMessagesAndTools(t *testing.T) {
	f := newFakeAPI(t, http.StatusOK, toolUseReply)
	if _, err := f.client().Next(context.Background(), conversation()); err != nil {
		t.Fatal(err)
	}
	if f.keys[0] != testKey {
		t.Errorf("X-Api-Key not sent")
	}
	got := f.only(t)
	want := map[string]any{}
	if err := json.Unmarshal([]byte(`{
		"model": "claude-test",
		"max_tokens": 4096,
		"system": [{"type":"text","text":"You are the agent."}],
		"messages": [
			{"role":"user","content":[{"type":"text","text":"Which services can I call?"}]},
			{"role":"assistant","content":[
				{"type":"text","text":"Let me look."},
				{"type":"tool_use","id":"toolu_1","name":"list_services","input":{}},
				{"type":"tool_use","id":"toolu_2","name":"list_operations","input":{"service":"nadi"}}]},
			{"role":"user","content":[
				{"type":"tool_result","tool_use_id":"toolu_1","content":[{"type":"text","text":"{\"organizations\":[]}"}],"is_error":false},
				{"type":"tool_result","tool_use_id":"toolu_2","content":[{"type":"text","text":"{\"error\":\"forbidden\",\"message\":\"no\"}"}],"is_error":true},
				{"type":"text","text":"Then list the devices."}]}
		],
		"tools": [{"name":"list_operations","description":"Lists the operations.","input_schema":
			{"type":"object","properties":{"service":{"type":"string","description":"Slug"}},"required":["service"],"additionalProperties":false}}]
	}`), &want); err != nil {
		t.Fatal(err)
	}
	for k, w := range want {
		gb, _ := json.Marshal(got[k])
		wb, _ := json.Marshal(w)
		if string(gb) != string(wb) {
			t.Errorf("%s:\n got %s\nwant %s", k, gb, wb)
		}
	}
}

func TestNextMapsToolUseToToolCalls(t *testing.T) {
	f := newFakeAPI(t, http.StatusOK, toolUseReply)
	reply, err := f.client().Next(context.Background(), conversation())
	if err != nil {
		t.Fatal(err)
	}
	if reply.Text != "Checking." {
		t.Errorf("text = %q", reply.Text)
	}
	if len(reply.Calls) != 1 {
		t.Fatalf("calls = %+v", reply.Calls)
	}
	c := reply.Calls[0]
	if c.ID != "toolu_9" || c.Name != "call_operation" {
		t.Errorf("call = %+v", c)
	}
	var in map[string]string
	if err := json.Unmarshal(c.Input, &in); err != nil || in["service"] != "nadi" || in["operation"] != "list_devices" {
		t.Errorf("input = %s", c.Input)
	}
}

func TestNextWithoutToolUseEndsTheTurn(t *testing.T) {
	f := newFakeAPI(t, http.StatusOK, `{"id":"msg_2","type":"message","role":"assistant","model":"claude-test",
		"content":[{"type":"text","text":"Two devices."}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`)
	reply, err := f.client().Next(context.Background(), conversation())
	if err != nil {
		t.Fatal(err)
	}
	if reply.Text != "Two devices." || len(reply.Calls) != 0 {
		t.Errorf("reply = %+v", reply)
	}
}

func TestNextMapsErrorsWithoutTheKey(t *testing.T) {
	cases := []struct {
		status int
		typ    string
		want   error
	}{
		{429, "rate_limit_error", domain.ErrModelUnavailable},
		{500, "api_error", domain.ErrModelUnavailable},
		{529, "overloaded_error", domain.ErrModelUnavailable},
		{401, "authentication_error", domain.ErrModelRefused},
		{400, "invalid_request_error", domain.ErrModelRefused},
		{404, "not_found_error", domain.ErrModelRefused},
	}
	for _, tc := range cases {
		// The body echoes the key, as a careless proxy might: it must not reach the error.
		body := `{"type":"error","error":{"type":"` + tc.typ + `","message":"key ` + testKey + `"}}`
		f := newFakeAPI(t, tc.status, body)
		_, err := f.client().Next(context.Background(), conversation())
		if !errors.Is(err, tc.want) {
			t.Errorf("HTTP %d: err = %v, want %v", tc.status, err, tc.want)
			continue
		}
		if strings.Contains(err.Error(), testKey) || strings.Contains(err.Error(), "SECRET") {
			t.Errorf("HTTP %d: the error holds the key: %v", tc.status, err)
		}
	}
}

func TestNextWithoutAnAnswerIsUnavailable(t *testing.T) {
	f := newFakeAPI(t, http.StatusOK, toolUseReply)
	f.srv.Close()
	_, err := f.client().Next(context.Background(), conversation())
	if !errors.Is(err, domain.ErrModelUnavailable) || strings.Contains(err.Error(), testKey) {
		t.Errorf("err = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = newFakeAPI(t, http.StatusOK, toolUseReply).client().Next(ctx, conversation())
	if !errors.Is(err, domain.ErrModelUnavailable) || !errors.Is(err, context.Canceled) {
		t.Errorf("canceled: err = %v", err)
	}
}
