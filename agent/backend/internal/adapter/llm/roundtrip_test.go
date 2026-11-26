// Package llm_test checks that a conversation stored in the neutral form works with every provider.
package llm_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	anthropicllm "github.com/nice-platform/example/agent/backend/internal/adapter/llm/anthropic"
	"github.com/nice-platform/example/agent/backend/internal/adapter/llm/openaicompat"
	chatdto "github.com/nice-platform/example/agent/backend/internal/app/chat/dto"
	chatusecase "github.com/nice-platform/example/agent/backend/internal/app/chat/usecase"
	"github.com/nice-platform/example/agent/backend/internal/domain"
)

// provider is a fake API that first asks for list_operations, then answers with text, and records what it got.
type provider struct {
	mu     sync.Mutex
	bodies []string
}

func (p *provider) record(r *http.Request) int {
	raw, _ := io.ReadAll(r.Body)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.bodies = append(p.bodies, string(raw))
	return len(p.bodies)
}

func (p *provider) last() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.bodies[len(p.bodies)-1]
}

func newAnthropic(t *testing.T, callID string) (chatusecase.Model, *provider) {
	p := &provider{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := p.record(r)
		w.Header().Set("Content-Type", "application/json")
		content := `[{"type":"text","text":"Done."}]`
		if n == 1 {
			content = `[{"type":"tool_use","id":"` + callID + `","name":"list_operations","input":{"service":"nadi"}}]`
		}
		_, _ = io.WriteString(w, `{"id":"msg","type":"message","role":"assistant","model":"m","content":`+content+
			`,"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`)
	}))
	t.Cleanup(srv.Close)
	return anthropicllm.New(anthropicllm.Options{Model: "m", BaseURL: srv.URL, APIKey: "k", HTTPClient: srv.Client()}), p
}

func newOpenAI(t *testing.T, callID string) (chatusecase.Model, *provider) {
	p := &provider{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := p.record(r)
		w.Header().Set("Content-Type", "application/json")
		msg := `{"role":"assistant","content":"Done."}`
		if n == 1 {
			msg = `{"role":"assistant","content":null,"tool_calls":[{"id":"` + callID +
				`","type":"function","function":{"name":"list_operations","arguments":"{\"service\":\"nadi\"}"}}]}`
		}
		_, _ = io.WriteString(w, `{"id":"c","object":"chat.completion","created":1,"model":"m","choices":[{"index":0,"finish_reason":"stop","message":`+msg+`}]}`)
	}))
	t.Cleanup(srv.Close)
	return openaicompat.New(openaicompat.Options{Model: "m", BaseURL: srv.URL, HTTPClient: srv.Client()}), p
}

// turn runs one turn the way the chat use case does and returns the conversation in the neutral form.
func turn(t *testing.T, m chatusecase.Model) []chatdto.Message {
	t.Helper()
	msgs := []chatdto.Message{{Role: domain.RoleUser, Parts: []chatdto.Part{{Kind: domain.PartText, Text: "What can nadi do?"}}}}
	for range 2 {
		reply, err := m.Next(context.Background(), chatdto.ModelRequest{System: "You are the agent.", Messages: msgs})
		if err != nil {
			t.Fatal(err)
		}
		var parts []chatdto.Part
		if reply.Text != "" {
			parts = append(parts, chatdto.Part{Kind: domain.PartText, Text: reply.Text})
		}
		var results []chatdto.Part
		for _, c := range reply.Calls {
			parts = append(parts, chatdto.Part{Kind: domain.PartToolCall, Call: &c})
			results = append(results, chatdto.Part{Kind: domain.PartToolResult, Result: &chatdto.ToolResult{
				CallID: c.ID, Content: json.RawMessage(`{"operations":["list_devices"]}`),
			}})
		}
		msgs = append(msgs, chatdto.Message{Role: domain.RoleAssistant, Parts: parts})
		if len(results) == 0 {
			return msgs
		}
		msgs = append(msgs, chatdto.Message{Role: domain.RoleTool, Parts: results})
	}
	t.Fatal("the turn did not end")
	return nil
}

func TestANeutralConversationWorksWithEveryProvider(t *testing.T) {
	a, ap := newAnthropic(t, "toolu_1")
	o, op := newOpenAI(t, "call_1")

	fromAnthropic := turn(t, a)
	fromOpenAI := turn(t, o)
	if len(fromAnthropic) != 4 || len(fromOpenAI) != 4 {
		t.Fatalf("turns: anthropic %d messages, openai %d", len(fromAnthropic), len(fromOpenAI))
	}
	// The result goes back with the ID of the call.
	if !strings.Contains(ap.last(), `"tool_use_id":"toolu_1"`) {
		t.Errorf("anthropic: tool_result without the call ID: %s", ap.last())
	}
	if !strings.Contains(op.last(), `"tool_call_id":"call_1"`) {
		t.Errorf("openai: tool message without the call ID: %s", op.last())
	}

	// The stored form does not depend on the provider: the same messages apart from the IDs the provider chose.
	norm := func(ms []chatdto.Message, id string) string {
		b, _ := json.Marshal(ms)
		return strings.ReplaceAll(string(b), id, "ID")
	}
	if a, o := norm(fromAnthropic, "toolu_1"), norm(fromOpenAI, "call_1"); a != o {
		t.Errorf("neutral forms differ:\nanthropic %s\nopenai    %s", a, o)
	}

	// A conversation begun with one provider continues with the other.
	ctx := context.Background()
	a2, ap2 := newAnthropic(t, "unused")
	if _, err := a2.Next(ctx, chatdto.ModelRequest{Messages: fromOpenAI}); err != nil {
		t.Fatal(err)
	}
	if b := ap2.last(); !strings.Contains(b, `"id":"call_1"`) || !strings.Contains(b, `"tool_use_id":"call_1"`) {
		t.Errorf("anthropic did not get the openai conversation: %s", b)
	}
	o2, op2 := newOpenAI(t, "unused")
	if _, err := o2.Next(ctx, chatdto.ModelRequest{Messages: fromAnthropic}); err != nil {
		t.Fatal(err)
	}
	if b := op2.last(); !strings.Contains(b, `"id":"toolu_1"`) || !strings.Contains(b, `"tool_call_id":"toolu_1"`) {
		t.Errorf("openai did not get the anthropic conversation: %s", b)
	}
}
