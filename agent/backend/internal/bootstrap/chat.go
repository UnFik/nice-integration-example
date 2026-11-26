package bootstrap

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	anthropicllm "github.com/nice-platform/example/agent/backend/internal/adapter/llm/anthropic"
	"github.com/nice-platform/example/agent/backend/internal/adapter/llm/openaicompat"
	mcpadapter "github.com/nice-platform/example/agent/backend/internal/adapter/mcp"
	chatmemory "github.com/nice-platform/example/agent/backend/internal/adapter/memory/chat"
	chatpostgres "github.com/nice-platform/example/agent/backend/internal/adapter/postgres/chat"
	"github.com/nice-platform/example/agent/backend/internal/adapter/rest/auth"
	restchat "github.com/nice-platform/example/agent/backend/internal/adapter/rest/chat"
	calldto "github.com/nice-platform/example/agent/backend/internal/app/call/dto"
	callusecase "github.com/nice-platform/example/agent/backend/internal/app/call/usecase"
	chatdto "github.com/nice-platform/example/agent/backend/internal/app/chat/dto"
	chatusecase "github.com/nice-platform/example/agent/backend/internal/app/chat/usecase"
	"github.com/nice-platform/example/agent/backend/internal/platform/config"
	"github.com/nice-platform/example/agent/backend/internal/platform/postgres"
	"github.com/nice-platform/example/agent/backend/migrations"
)

// chatTools runs the agent's tools for the chat through the same call use cases as the MCP tools. A success is the
// JSON an MCP client gets as structuredContent; a failure carries the same code and message as the MCP error text.
type chatTools struct {
	calls *callusecase.Service
	specs []chatdto.ToolSpec
}

// NewChatTools returns the tools of the chat over calls, described as the MCP server describes them.
func NewChatTools(calls *callusecase.Service) chatusecase.Tools {
	return &chatTools{calls: calls, specs: mcpadapter.ToolSpecs()}
}

func (t *chatTools) Specs() []chatdto.ToolSpec { return t.specs }

// The field names are those of the MCP tool inputs, so the model's input is read the same way.
type chatToolInput struct {
	Service        string            `json:"service"`
	Operation      string            `json:"operation"`
	OrganizationID string            `json:"organization_id"`
	ProjectID      string            `json:"project_id"`
	PathParams     map[string]string `json:"path_params"`
	Query          map[string]string `json:"query"`
	Body           any               `json:"body"`
}

func (t *chatTools) Run(ctx context.Context, c calldto.Caller, call chatdto.ToolCall, ap *calldto.Approval) chatdto.ToolOutcome {
	var in chatToolInput
	if len(bytes.TrimSpace(call.Input)) > 0 {
		dec := json.NewDecoder(bytes.NewReader(call.Input))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&in); err != nil {
			return failed(&calldto.Failure{Code: "invalid_argument", Message: "the input does not match the tool's input schema"})
		}
	}
	switch call.Name {
	case "list_services":
		out, f := t.calls.ListServices(ctx, c)
		return result(out, f)
	case "list_operations":
		out, f := t.calls.ListOperations(ctx, c, in.Service)
		return result(out, f)
	case "call_operation":
		answer := calldto.Approval{}
		if ap != nil {
			answer = *ap
		}
		out, ask, f := t.calls.CallOperation(ctx, c, calldto.OperationInput{
			Service: in.Service, Operation: in.Operation,
			Where:      calldto.Where{Org: in.OrganizationID, Project: in.ProjectID},
			PathParams: in.PathParams, Query: in.Query, Body: in.Body,
		}, answer)
		if ask != nil {
			return chatdto.ToolOutcome{Approval: ask}
		}
		return result(out, f)
	}
	return failed(&calldto.Failure{Code: "invalid_argument", Message: fmt.Sprintf("unknown tool %q", call.Name)})
}

func result[T any](out *T, f *calldto.Failure) chatdto.ToolOutcome {
	if f != nil {
		return failed(f)
	}
	b, err := json.Marshal(out)
	if err != nil {
		return failed(&calldto.Failure{Code: "internal", Message: "the result could not be encoded"})
	}
	return chatdto.ToolOutcome{Content: b}
}

func failed(f *calldto.Failure) chatdto.ToolOutcome {
	v := map[string]string{"error": f.Code, "message": f.Message}
	if f.Description != "" && f.Description != f.Code {
		v["description"] = f.Description
	}
	b, _ := json.Marshal(v)
	return chatdto.ToolOutcome{Content: b, IsError: true}
}

// NewModel returns the chat model of cfg, or nil when no provider is set: the chat is then off.
func NewModel(l config.LLM, hc *http.Client) chatusecase.Model {
	switch l.Provider {
	case config.ProviderAnthropic:
		return anthropicllm.New(anthropicllm.Options{Model: l.Model, BaseURL: l.BaseURL, APIKey: l.APIKey, HTTPClient: hc})
	case config.ProviderOpenAI:
		return openaicompat.New(openaicompat.Options{Model: l.Model, BaseURL: l.BaseURL, APIKey: l.APIKey, HTTPClient: hc})
	}
	return nil
}

// NewConversations returns the conversation store: PostgreSQL with its migrations applied when DatabaseURL is set,
// memory otherwise. The returned close releases the pool.
func NewConversations(ctx context.Context, cfg config.Config) (chatusecase.Conversations, func(), error) {
	if cfg.DatabaseURL == "" {
		return chatmemory.New(), func() {}, nil
	}
	pool, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, nil, err
	}
	if err := postgres.Migrate(ctx, pool, migrations.FS); err != nil {
		pool.Close()
		return nil, nil, err
	}
	return chatpostgres.New(pool), func() { closePool(pool) }, nil
}

func closePool(p *pgxpool.Pool) { p.Close() }

// NewChatService assembles the chat use case.
func NewChatService(cfg config.Config, store chatusecase.Conversations, calls *callusecase.Service, hc *http.Client, log *slog.Logger) *chatusecase.Service {
	return &chatusecase.Service{Conversations: store, Model: NewModel(cfg.LLM, hc), Tools: NewChatTools(calls), MaxSteps: cfg.ChatMaxSteps, Log: log}
}

// NewChatHandler serves the chat API for the user the session middleware put in the request.
func NewChatHandler(cfg config.Config, svc *chatusecase.Service, log *slog.Logger) http.Handler {
	h := &restchat.Handler{Chat: svc, ChatEnabled: cfg.ChatEnabled(), Log: log,
		Caller: func(r *http.Request) (calldto.Caller, bool) { return auth.CallerFrom(r.Context()) }}
	return h.Routes()
}
