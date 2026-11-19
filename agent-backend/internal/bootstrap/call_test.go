package bootstrap

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	calldto "github.com/nice-platform/example/agent-backend/internal/app/call/dto"
	callusecase "github.com/nice-platform/example/agent-backend/internal/app/call/usecase"
	"github.com/nice-platform/example/agent-backend/internal/domain"
	"github.com/nice-platform/example/agent-backend/internal/platform/config"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nice-platform/nice/pkg/niceauth/verify"
)

// callEnv is the agent without the MCP layer, for the calls themselves.
type callEnv struct {
	*testEnv
	calls *callusecase.Service
}

func newCallEnv(t *testing.T) *callEnv {
	e := newTestEnv(t)
	calls, err := NewCallService(e.cfg, http.DefaultClient, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	return &callEnv{testEnv: e, calls: calls}
}

// call runs the use case and shows a failure the way the MCP adapter does, so the tests read what the model reads.
func (e *callEnv) call(ctx context.Context, c calldto.Caller, t domain.Target, where calldto.Where, method, path string, body any) (*mcp.CallToolResult, any) {
	out, f := e.calls.ExecuteCall(ctx, c, t, where, method, path, body)
	if f != nil {
		text := f.Code
		if f.Description != "" && f.Description != f.Code {
			text += " (" + f.Description + ")"
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text + ": " + f.Message}}, IsError: true}, nil
	}
	return nil, out
}

func (e *callEnv) caller(org, prj string) calldto.Caller {
	tok := e.userToken(org, prj)
	v, err := verify.New(verify.Config{Issuer: e.issuer, Service: domain.ServiceSlug, StaticJWKS: testJWKS(e.t), Log: slog.New(slog.DiscardHandler)})
	if err != nil {
		e.t.Fatal(err)
	}
	c, err := v.Verify(context.Background(), tok)
	if err != nil {
		e.t.Fatal(err)
	}
	return calldto.Caller{Token: tok, Subject: c.Subject, OrgID: c.OrgID}
}

// resultText is the text of a tool result, and fails when it shows a token or the agent's secret.
func resultText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	raw, _ := json.Marshal(res)
	if strings.Contains(string(raw), "eyJ") || strings.Contains(string(raw), testSecret) {
		t.Fatalf("the tool result shows a token or the secret: %s", raw)
	}
	return b.String()
}

func wantToolError(t *testing.T, res *mcp.CallToolResult, out any, code string) string {
	t.Helper()
	if res == nil || !res.IsError {
		t.Fatalf("result = %+v %v, want an error naming %s", res, out, code)
	}
	text := resultText(t, res)
	if !strings.Contains(text, code) {
		t.Fatalf("error text %q does not name %s", text, code)
	}
	return text
}

func wantOK(t *testing.T, res *mcp.CallToolResult, out any) map[string]any {
	t.Helper()
	if res != nil && res.IsError {
		t.Fatalf("result is an error: %s", resultText(t, res))
	}
	m, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("output = %T %v, want a JSON object", out, out)
	}
	return m
}

func TestCallExchangesTheUserTokenAndSendsOnlyTheExchangedOne(t *testing.T) {
	e := newCallEnv(t)
	c := e.caller(orgA, prjP)
	res, out := e.call(context.Background(), c, domain.Inventaris, calldto.Where{}, http.MethodGet, "/items", nil)
	got := wantOK(t, res, out)
	if got["org"] != orgA || got["project"] != prjP {
		t.Fatalf("inventaris saw context %v/%v, want the token's %s/%s", got["org"], got["project"], orgA, prjP)
	}
	ex := e.nice.Exchanges()
	if len(ex) != 1 || ex[0].Scope != "nice:aud:inventaris" {
		t.Fatalf("exchanges = %+v, want one for nice:aud:inventaris with the token's context", ex)
	}
	rec := e.inv.Received()
	if len(rec) != 1 {
		t.Fatalf("inventaris received %d requests", len(rec))
	}
	if rec[0]["aud"].([]any)[0] != "inventaris" || rec[0]["sub"] != testUser || rec[0]["act"].(map[string]any)["sub"] != testClientID {
		t.Fatalf("inventaris received claims %v", rec[0])
	}
}

func TestCallRefusedWithAccessDeniedCallsNoService(t *testing.T) {
	e := newCallEnv(t)
	res, out := e.call(context.Background(), e.caller(orgA, prjP), domain.Inventaris, calldto.Where{Org: orgC, Project: prjQ},
		http.MethodGet, "/items", nil)
	text := wantToolError(t, res, out, "access_denied")
	if !strings.Contains(text, "organization") {
		t.Fatalf("text %q does not explain the reason", text)
	}
	if n := len(e.inv.Received()); n != 0 {
		t.Fatalf("inventaris was called %d times", n)
	}
}

func TestCallRefusedWithInvalidScope(t *testing.T) {
	e := newCallEnv(t)
	e.nice.Refuse("inventaris", 400, "invalid_scope", "inventaris is not active in the project")
	res, out := e.call(context.Background(), e.caller(orgA, prjP), domain.Inventaris, calldto.Where{}, http.MethodGet, "/items", nil)
	wantToolError(t, res, out, "invalid_scope")
	if n := len(e.inv.Received()); n != 0 {
		t.Fatalf("inventaris was called %d times", n)
	}
}

func TestCallExplainsAnExpiringUserToken(t *testing.T) {
	e := newCallEnv(t)
	e.nice.Refuse("inventaris", 400, "invalid_grant", "subject_token_expiring: the subject token expires within 60 seconds")
	res, out := e.call(context.Background(), e.caller(orgA, prjP), domain.Inventaris, calldto.Where{}, http.MethodGet, "/items", nil)
	text := wantToolError(t, res, out, "subject_token_expiring")
	if !strings.Contains(text, "refresh") {
		t.Fatalf("text %q does not say to refresh the token", text)
	}
}

func TestCallExplainsARefusedAgentClient(t *testing.T) {
	e := newCallEnv(t)
	e.nice.Refuse("inventaris", 401, "invalid_client", "client authentication failed")
	res, out := e.call(context.Background(), e.caller(orgA, prjP), domain.Inventaris, calldto.Where{}, http.MethodGet, "/items", nil)
	text := wantToolError(t, res, out, "invalid_client")
	if !strings.Contains(text, config.EnvClientSecret) {
		t.Fatalf("text %q does not point the operator to the client", text)
	}
}

func TestCallNamesPermissionDeniedOfTheService(t *testing.T) {
	e := newCallEnv(t)
	e.nice.perms["inventaris"] = []string{"inventaris.items.read"}
	res, out := e.call(context.Background(), e.caller(orgA, prjP), domain.Inventaris, calldto.Where{}, http.MethodPost, "/items",
		map[string]any{"name": "Kabel", "quantity": 1})
	text := wantToolError(t, res, out, "permission_denied")
	if !strings.Contains(text, "inventaris") {
		t.Fatalf("text %q does not name the service", text)
	}
}

func TestCallRetriesOnceWhenTheServiceSaysTokenStale(t *testing.T) {
	e := newCallEnv(t)
	e.inv.StaleOnce()
	res, out := e.call(context.Background(), e.caller(orgA, prjP), domain.Inventaris, calldto.Where{}, http.MethodPost, "/items",
		map[string]any{"name": "Kabel", "quantity": 2})
	got := wantOK(t, res, out)
	if got["name"] != "Kabel" {
		t.Fatalf("item = %v", got)
	}
	if n := len(e.nice.Exchanges()); n != 2 {
		t.Fatalf("exchanges = %d, want 2 (one fresh exchange after token_stale)", n)
	}
	if n := len(e.inv.Received()); n != 2 {
		t.Fatalf("inventaris received %d requests, want 2", n)
	}
}

func TestCallReportsTokenStaleWhenItPersists(t *testing.T) {
	e := newCallEnv(t)
	e.inv.StaleOnce()
	// The second answer is stale too: DoAs returns it, and the tool says so.
	e.nice.onExchange = func(string) { e.inv.StaleOnce() }
	res, out := e.call(context.Background(), e.caller(orgA, prjP), domain.Inventaris, calldto.Where{}, http.MethodGet, "/items", nil)
	wantToolError(t, res, out, "token_stale")
	if n := len(e.inv.Received()); n != 2 {
		t.Fatalf("inventaris received %d requests, want 2 (one retry only)", n)
	}
}

func TestCallWhenNICEDoesNotAnswer(t *testing.T) {
	e := newCallEnv(t)
	e.nice.srv.Close()
	res, out := e.call(context.Background(), e.caller(orgA, prjP), domain.Inventaris, calldto.Where{}, http.MethodGet, "/items", nil)
	wantToolError(t, res, out, "nice_unavailable")
}
