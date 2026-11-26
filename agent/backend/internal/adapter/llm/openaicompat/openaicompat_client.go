// Package openaicompat is the chat model on an OpenAI-compatible Chat Completions API (OpenAI, Ollama, vLLM, and
// other servers that speak it) with function tools.
package openaicompat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	chatdto "github.com/nice-platform/example/agent/backend/internal/app/chat/dto"
	"github.com/nice-platform/example/agent/backend/internal/domain"
)

const (
	// DefaultBaseURL is the OpenAI API; a local server such as Ollama is e.g. http://localhost:11434/v1.
	DefaultBaseURL = "https://api.openai.com/v1/"
	defaultRetries = 2
	// A local model may take minutes for one reply.
	defaultTimeout = 5 * time.Minute
)

// Options configures the client. BaseURL empty means DefaultBaseURL. APIKey may be empty for a server that needs none:
// the request then carries no Authorization header. HTTPClient nil means a client with a 5 minute timeout.
type Options struct {
	Model      string
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// Client implements the chat use case's Model.
type Client struct {
	completions openai.ChatCompletionService
	model       string
}

// New builds the client. It reads nothing from the environment: OPENAI_* variables on the host must not override
// what the agent was configured with, which is why it builds the completions service instead of openai.NewClient.
func New(o Options) *Client { return newClient(o, defaultRetries) }

func newClient(o Options, retries int) *Client {
	base := o.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	hc := o.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: defaultTimeout}
	}
	opts := []option.RequestOption{
		option.WithHTTPClient(hc),
		option.WithBaseURL(base),
		option.WithMaxRetries(retries),
	}
	if o.APIKey != "" {
		opts = append(opts, option.WithAPIKey(o.APIKey))
	}
	return &Client{completions: openai.NewChatCompletionService(opts...), model: o.Model}
}

// Next sends the conversation and returns the text and tool calls of the reply.
func (c *Client) Next(ctx context.Context, req chatdto.ModelRequest) (chatdto.ModelReply, error) {
	tools, err := toTools(req.Tools)
	if err != nil {
		return chatdto.ModelReply{}, fmt.Errorf("%w: openai-compatible: %v", domain.ErrModelRefused, err)
	}
	params := openai.ChatCompletionNewParams{
		Model:    c.model,
		Messages: toMessages(req.System, req.Messages),
		Tools:    tools,
	}
	res, err := c.completions.New(ctx, params)
	if err != nil {
		return chatdto.ModelReply{}, mapError(ctx, err)
	}
	if len(res.Choices) == 0 {
		return chatdto.ModelReply{}, fmt.Errorf("%w: openai-compatible: a reply without choices", domain.ErrModelUnavailable)
	}
	return fromMessage(res.Choices[0].Message), nil
}

func toMessages(system string, ms []chatdto.Message) []openai.ChatCompletionMessageParamUnion {
	var out []openai.ChatCompletionMessageParamUnion
	if system != "" {
		out = append(out, openai.SystemMessage(system))
	}
	for _, m := range ms {
		switch m.Role {
		case domain.RoleUser:
			if text := joinText(m.Parts); text != "" {
				out = append(out, openai.UserMessage(text))
			}
		case domain.RoleAssistant:
			var a openai.ChatCompletionAssistantMessageParam
			if text := joinText(m.Parts); text != "" {
				a.Content.OfString = openai.String(text)
			}
			for _, p := range m.Parts {
				if p.Kind != domain.PartToolCall || p.Call == nil {
					continue
				}
				a.ToolCalls = append(a.ToolCalls, openai.ChatCompletionMessageToolCallUnionParam{
					OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
						ID: p.Call.ID,
						Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
							Name: p.Call.Name, Arguments: string(objectOrEmpty(p.Call.Input)),
						},
					},
				})
			}
			if a.Content.OfString.Valid() || len(a.ToolCalls) > 0 {
				out = append(out, openai.ChatCompletionMessageParamUnion{OfAssistant: &a})
			}
		case domain.RoleTool:
			for _, p := range m.Parts {
				if p.Kind == domain.PartToolResult && p.Result != nil {
					out = append(out, openai.ToolMessage(resultContent(p.Result), p.Result.CallID))
				}
			}
		}
	}
	return out
}

func joinText(parts []chatdto.Part) string {
	var text string
	for _, p := range parts {
		if p.Kind == domain.PartText && p.Text != "" {
			if text != "" {
				text += "\n\n"
			}
			text += p.Text
		}
	}
	return text
}

// resultContent marks a failed call inside the content: Chat Completions has no is_error on a tool message.
func resultContent(r *chatdto.ToolResult) string {
	content := objectOrEmpty(r.Content)
	if !r.IsError {
		return string(content)
	}
	b, _ := json.Marshal(struct {
		IsError bool            `json:"is_error"`
		Content json.RawMessage `json:"content"`
	}{true, content})
	return string(b)
}

func objectOrEmpty(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage(`{}`)
	}
	return raw
}

func toTools(specs []chatdto.ToolSpec) ([]openai.ChatCompletionToolUnionParam, error) {
	out := make([]openai.ChatCompletionToolUnionParam, 0, len(specs))
	for _, s := range specs {
		var schema map[string]any
		if err := json.Unmarshal(s.InputSchema, &schema); err != nil {
			return nil, fmt.Errorf("input schema of tool %s: %v", s.Name, err)
		}
		fn := openai.FunctionDefinitionParam{Name: s.Name, Parameters: openai.FunctionParameters(schema)}
		if s.Description != "" {
			fn.Description = openai.String(s.Description)
		}
		out = append(out, openai.ChatCompletionFunctionTool(fn))
	}
	return out, nil
}

func fromMessage(m openai.ChatCompletionMessage) chatdto.ModelReply {
	reply := chatdto.ModelReply{Text: m.Content}
	for _, tc := range m.ToolCalls {
		if tc.Type != "" && tc.Type != "function" {
			continue
		}
		reply.Calls = append(reply.Calls, chatdto.ToolCall{
			ID: tc.ID, Name: tc.Function.Name, Input: arguments(tc.Function.Arguments),
		})
	}
	return reply
}

// arguments keeps the call when the model wrote invalid JSON: {"_raw": "..."} fails the input check of the tool, and
// the error goes back to the model, which can correct itself, instead of failing the whole turn.
func arguments(s string) json.RawMessage {
	if s == "" {
		return json.RawMessage(`{}`)
	}
	if json.Valid([]byte(s)) {
		return json.RawMessage(s)
	}
	b, _ := json.Marshal(map[string]string{"_raw": s})
	return b
}

// mapError builds its own message: the text of a provider error may echo part of the key (OpenAI does on 401), and
// the key must never reach a log or an answer.
func mapError(ctx context.Context, err error) error {
	var apiErr *openai.Error
	if errors.As(err, &apiErr) {
		kind := domain.ErrModelRefused
		if retryable(apiErr.StatusCode) {
			kind = domain.ErrModelUnavailable
		}
		return fmt.Errorf("%w: openai-compatible: HTTP %d", kind, apiErr.StatusCode)
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return fmt.Errorf("%w: openai-compatible: %w", domain.ErrModelUnavailable, ctxErr)
	}
	return fmt.Errorf("%w: openai-compatible: no answer from the API", domain.ErrModelUnavailable)
}

func retryable(status int) bool {
	switch status {
	case http.StatusRequestTimeout, http.StatusConflict, http.StatusTooManyRequests:
		return true
	}
	return status >= 500
}
