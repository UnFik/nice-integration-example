package openaicompat

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

const testKey = "sk-test-SECRET-0123456789"

type fakeAPI struct {
	srv    *httptest.Server
	mu     sync.Mutex
	bodies []map[string]any
	auth   []string
}

func newFakeAPI(t *testing.T, status int, reply string) *fakeAPI {
	t.Helper()
	f := &fakeAPI{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		f.mu.Lock()
		f.bodies = append(f.bodies, body)
		f.auth = append(f.auth, r.Header.Get("Authorization"))
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, reply)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeAPI) client(key string) *Client {
	return newClient(Options{Model: "gpt-test", BaseURL: f.srv.URL + "/v1", APIKey: key, HTTPClient: f.srv.Client()}, 0)
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
				{Kind: domain.PartToolCall, Call: &chatdto.ToolCall{ID: "call_1", Name: "list_services", Input: json.RawMessage(`{}`)}},
				{Kind: domain.PartToolCall, Call: &chatdto.ToolCall{ID: "call_2", Name: "list_operations", Input: json.RawMessage(`{"service":"nadi"}`)}},
			}},
			{Role: domain.RoleTool, Parts: []chatdto.Part{
				{Kind: domain.PartToolResult, Result: &chatdto.ToolResult{CallID: "call_1", Content: json.RawMessage(`{"organizations":[]}`)}},
				{Kind: domain.PartToolResult, Result: &chatdto.ToolResult{CallID: "call_2", Content: json.RawMessage(`{"error":"forbidden","message":"no"}`), IsError: true}},
			}},
			{Role: domain.RoleUser, Parts: []chatdto.Part{{Kind: domain.PartText, Text: "Then list the devices."}}},
		},
		Tools: []chatdto.ToolSpec{{
			Name: "list_operations", Description: "Lists the operations.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"service":{"type":"string","description":"Slug"}},"required":["service"],"additionalProperties":false}`),
		}},
	}
}

func completion(message string) string {
	return `{"id":"chatcmpl-1","object":"chat.completion","created":1,"model":"gpt-test",
"choices":[{"index":0,"finish_reason":"tool_calls","logprobs":null,"message":` + message + `}]}`
}

var toolCallsReply = completion(`{"role":"assistant","content":"Checking.","refusal":null,
"tool_calls":[{"id":"call_9","type":"function","function":{"name":"call_operation","arguments":"{\"service\":\"nadi\",\"operation\":\"list_devices\"}"}}]}`)

func TestNextSendsSystemMessagesAndTools(t *testing.T) {
	f := newFakeAPI(t, http.StatusOK, toolCallsReply)
	if _, err := f.client(testKey).Next(context.Background(), conversation()); err != nil {
		t.Fatal(err)
	}
	if f.auth[0] != "Bearer "+testKey {
		t.Errorf("Authorization not sent")
	}
	got := f.only(t)
	want := map[string]any{}
	if err := json.Unmarshal([]byte(`{
		"model": "gpt-test",
		"messages": [
			{"role":"system","content":"You are the agent."},
			{"role":"user","content":"Which services can I call?"},
			{"role":"assistant","content":"Let me look.","tool_calls":[
				{"id":"call_1","type":"function","function":{"name":"list_services","arguments":"{}"}},
				{"id":"call_2","type":"function","function":{"name":"list_operations","arguments":"{\"service\":\"nadi\"}"}}]},
			{"role":"tool","tool_call_id":"call_1","content":"{\"organizations\":[]}"},
			{"role":"tool","tool_call_id":"call_2","content":"{\"is_error\":true,\"content\":{\"error\":\"forbidden\",\"message\":\"no\"}}"},
			{"role":"user","content":"Then list the devices."}
		],
		"tools": [{"type":"function","function":{"name":"list_operations","description":"Lists the operations.","parameters":
			{"type":"object","properties":{"service":{"type":"string","description":"Slug"}},"required":["service"],"additionalProperties":false}}}]
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

func TestNextWithoutAKeySendsNoAuthorization(t *testing.T) {
	f := newFakeAPI(t, http.StatusOK, toolCallsReply)
	if _, err := f.client("").Next(context.Background(), conversation()); err != nil {
		t.Fatal(err)
	}
	if f.auth[0] != "" {
		t.Errorf("Authorization = %q, want none", f.auth[0])
	}
}

func TestNextMapsToolCallsWithJSONStringArguments(t *testing.T) {
	f := newFakeAPI(t, http.StatusOK, toolCallsReply)
	reply, err := f.client(testKey).Next(context.Background(), conversation())
	if err != nil {
		t.Fatal(err)
	}
	if reply.Text != "Checking." || len(reply.Calls) != 1 {
		t.Fatalf("reply = %+v", reply)
	}
	c := reply.Calls[0]
	if c.ID != "call_9" || c.Name != "call_operation" {
		t.Errorf("call = %+v", c)
	}
	var in map[string]string
	if err := json.Unmarshal(c.Input, &in); err != nil || in["service"] != "nadi" || in["operation"] != "list_devices" {
		t.Errorf("input = %s", c.Input)
	}
}

func TestNextKeepsInvalidArgumentsForTheToolToRefuse(t *testing.T) {
	f := newFakeAPI(t, http.StatusOK, completion(`{"role":"assistant","content":null,
"tool_calls":[{"id":"call_7","type":"function","function":{"name":"list_operations","arguments":"{\"service\": nadi"}}]}`))
	reply, err := f.client(testKey).Next(context.Background(), conversation())
	if err != nil {
		t.Fatal(err)
	}
	if len(reply.Calls) != 1 || reply.Text != "" {
		t.Fatalf("reply = %+v", reply)
	}
	var in map[string]string
	if err := json.Unmarshal(reply.Calls[0].Input, &in); err != nil || in["_raw"] != `{"service": nadi` {
		t.Errorf("input = %s", reply.Calls[0].Input)
	}
}

func TestNextMapsErrorsWithoutTheKey(t *testing.T) {
	cases := []struct {
		status int
		want   error
	}{
		{429, domain.ErrModelUnavailable},
		{500, domain.ErrModelUnavailable},
		{503, domain.ErrModelUnavailable},
		{401, domain.ErrModelRefused},
		{400, domain.ErrModelRefused},
		{404, domain.ErrModelRefused},
	}
	for _, tc := range cases {
		// OpenAI echoes part of a wrong key on 401; this body echoes all of it.
		body := `{"error":{"message":"Incorrect API key provided: ` + testKey + `","type":"invalid_request_error","param":null,"code":"invalid_api_key"}}`
		f := newFakeAPI(t, tc.status, body)
		_, err := f.client(testKey).Next(context.Background(), conversation())
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
	f := newFakeAPI(t, http.StatusOK, toolCallsReply)
	f.srv.Close()
	_, err := f.client(testKey).Next(context.Background(), conversation())
	if !errors.Is(err, domain.ErrModelUnavailable) || strings.Contains(err.Error(), testKey) {
		t.Errorf("err = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = newFakeAPI(t, http.StatusOK, toolCallsReply).client(testKey).Next(ctx, conversation())
	if !errors.Is(err, domain.ErrModelUnavailable) || !errors.Is(err, context.Canceled) {
		t.Errorf("canceled: err = %v", err)
	}
}
