// Package anthropicllm is the chat model on the Anthropic Messages API with tool use.
package anthropicllm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	chatdto "github.com/nice-platform/example/agent/backend/internal/app/chat/dto"
	"github.com/nice-platform/example/agent/backend/internal/domain"
)

const (
	maxTokens      = 4096
	defaultRetries = 2
)

// Options configures the client. BaseURL empty means the Anthropic API; HTTPClient nil means the SDK default.
type Options struct {
	Model      string
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// Client implements the chat use case's Model.
type Client struct {
	sdk   anthropic.Client
	model string
}

// New builds the client. It reads nothing from the environment: ANTHROPIC_* variables or a profile on the host must
// not override what the agent was configured with.
func New(o Options) *Client { return newClient(o, defaultRetries) }

func newClient(o Options, retries int) *Client {
	opts := []option.RequestOption{
		option.WithoutEnvironmentDefaults(),
		option.WithAPIKey(o.APIKey),
		option.WithMaxRetries(retries),
	}
	if o.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(o.BaseURL))
	}
	if o.HTTPClient != nil {
		opts = append(opts, option.WithHTTPClient(o.HTTPClient))
	}
	return &Client{sdk: anthropic.NewClient(opts...), model: o.Model}
}

// Next sends the conversation and returns the text and tool calls of the reply.
func (c *Client) Next(ctx context.Context, req chatdto.ModelRequest) (chatdto.ModelReply, error) {
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(c.model),
		MaxTokens: maxTokens,
		Messages:  toMessages(req.Messages),
	}
	if req.System != "" {
		params.System = []anthropic.TextBlockParam{{Text: req.System}}
	}
	tools, err := toTools(req.Tools)
	if err != nil {
		return chatdto.ModelReply{}, fmt.Errorf("%w: anthropic: %v", domain.ErrModelRefused, err)
	}
	params.Tools = tools

	msg, err := c.sdk.Messages.New(ctx, params)
	if err != nil {
		return chatdto.ModelReply{}, mapError(ctx, err)
	}
	return fromMessage(msg), nil
}

func toMessages(ms []chatdto.Message) []anthropic.MessageParam {
	var out []anthropic.MessageParam
	for _, m := range ms {
		var blocks []anthropic.ContentBlockParamUnion
		for _, p := range m.Parts {
			switch {
			case p.Kind == domain.PartText && p.Text != "":
				blocks = append(blocks, anthropic.NewTextBlock(p.Text))
			case p.Kind == domain.PartToolCall && p.Call != nil:
				blocks = append(blocks, anthropic.NewToolUseBlock(p.Call.ID, objectOrEmpty(p.Call.Input), p.Call.Name))
			case p.Kind == domain.PartToolResult && p.Result != nil:
				blocks = append(blocks, anthropic.NewToolResultBlock(p.Result.CallID, string(p.Result.Content), p.Result.IsError))
			}
		}
		if len(blocks) == 0 {
			continue
		}
		role := anthropic.MessageParamRoleUser
		if m.Role == domain.RoleAssistant {
			role = anthropic.MessageParamRoleAssistant
		}
		// Tool results travel in a user message; merging keeps user and assistant turns alternating when a user
		// message follows them.
		if n := len(out); n > 0 && out[n-1].Role == role {
			out[n-1].Content = append(out[n-1].Content, blocks...)
			continue
		}
		out = append(out, anthropic.MessageParam{Role: role, Content: blocks})
	}
	return out
}

func objectOrEmpty(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage(`{}`)
	}
	return raw
}

func toTools(specs []chatdto.ToolSpec) ([]anthropic.ToolUnionParam, error) {
	out := make([]anthropic.ToolUnionParam, 0, len(specs))
	for _, s := range specs {
		var schema map[string]any
		if err := json.Unmarshal(s.InputSchema, &schema); err != nil {
			return nil, fmt.Errorf("input schema of tool %s: %v", s.Name, err)
		}
		in := anthropic.ToolInputSchemaParam{Properties: schema["properties"]}
		if req, ok := schema["required"].([]any); ok {
			for _, r := range req {
				if name, ok := r.(string); ok {
					in.Required = append(in.Required, name)
				}
			}
		}
		delete(schema, "properties")
		delete(schema, "required")
		delete(schema, "type")
		if len(schema) > 0 {
			in.ExtraFields = schema
		}
		t := anthropic.ToolParam{Name: s.Name, InputSchema: in}
		if s.Description != "" {
			t.Description = anthropic.String(s.Description)
		}
		out = append(out, anthropic.ToolUnionParam{OfTool: &t})
	}
	return out, nil
}

func fromMessage(msg *anthropic.Message) chatdto.ModelReply {
	var reply chatdto.ModelReply
	var texts []string
	for _, b := range msg.Content {
		switch b.Type {
		case "text":
			if b.Text != "" {
				texts = append(texts, b.Text)
			}
		case "tool_use":
			reply.Calls = append(reply.Calls, chatdto.ToolCall{ID: b.ID, Name: b.Name, Input: objectOrEmpty(b.Input)})
		}
	}
	reply.Text = strings.Join(texts, "\n\n")
	return reply
}

// mapError builds its own message: the SDK error text carries the request URL and the response body, and the key
// must never reach a log or an answer.
func mapError(ctx context.Context, err error) error {
	var apiErr *anthropic.Error
	if errors.As(err, &apiErr) {
		kind := domain.ErrModelRefused
		if retryable(apiErr.StatusCode) {
			kind = domain.ErrModelUnavailable
		}
		if t := apiErr.Type(); t != "" {
			return fmt.Errorf("%w: anthropic: HTTP %d (%s)", kind, apiErr.StatusCode, t)
		}
		return fmt.Errorf("%w: anthropic: HTTP %d", kind, apiErr.StatusCode)
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return fmt.Errorf("%w: anthropic: %w", domain.ErrModelUnavailable, ctxErr)
	}
	return fmt.Errorf("%w: anthropic: no answer from the API", domain.ErrModelUnavailable)
}

func retryable(status int) bool {
	switch status {
	case http.StatusRequestTimeout, http.StatusConflict, http.StatusTooManyRequests:
		return true
	}
	return status >= 500
}
