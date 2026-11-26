package bootstrap

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	calldto "github.com/nice-platform/example/agent/backend/internal/app/call/dto"
	callusecase "github.com/nice-platform/example/agent/backend/internal/app/call/usecase"
	"github.com/nice-platform/example/agent/backend/internal/domain"
	"github.com/nice-platform/example/agent/backend/internal/platform/config"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/ABA-Developer/nice-auth-sdk/verify"
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

// call runs call_operation's use case for an operation of nadi and shows a failure the way the MCP adapter does, so
// the tests read what the model reads.
func (e *callEnv) call(ctx context.Context, c calldto.Caller, key string, where calldto.Where, body any) (*mcp.CallToolResult, any) {
	out, ask, f := e.calls.CallOperation(ctx, c, calldto.OperationInput{Service: "nadi", Operation: key, Where: where, Body: body}, calldto.Approval{})
	if ask != nil {
		e.t.Fatalf("%s asked for an approval", key)
	}
	if f != nil {
		text := f.Code
		if f.Description != "" && f.Description != f.Code {
			text += " (" + f.Description + ")"
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text + ": " + f.Message}}, IsError: true}, nil
	}
	return nil, out.Body
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
	return calldto.Caller{Token: tok, Subject: c.Subject, OrgID: c.OrgID, ProjectID: c.ProjectID}
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

var newDevice = map[string]any{"name": "Sensor", "serial_number": "SN-2", "status": "active"}

func TestCallExchangesTheUserTokenAndSendsOnlyTheExchangedOne(t *testing.T) {
	e := newCallEnv(t)
	res, out := e.call(context.Background(), e.caller(orgA, prjP), "listDevices", calldto.Where{}, nil)
	got := wantOK(t, res, out)
	if got["org"] != orgA || got["project"] != prjP {
		t.Fatalf("nadi saw context %v/%v, want the token's %s/%s", got["org"], got["project"], orgA, prjP)
	}
	ex := e.nice.ExchangesFor("nadi")
	if len(ex) != 1 || ex[0].Scope != "nice:aud:nadi" {
		t.Fatalf("exchanges = %+v, want one for nice:aud:nadi with the token's context", ex)
	}
	if n := len(e.nice.ExchangesFor("nice")); n != 1 {
		t.Fatalf("exchanges for the NICE API = %d, want 1 for the map and the operations", n)
	}
	rec := e.inv.Received()
	if len(rec) != 1 {
		t.Fatalf("nadi received %d requests", len(rec))
	}
	if rec[0]["aud"].([]any)[0] != "nadi" || rec[0]["sub"] != testUser || rec[0]["act"].(map[string]any)["sub"] != testClientID {
		t.Fatalf("nadi received claims %v", rec[0])
	}
}

func TestCallRefusedWithAccessDeniedCallsNoService(t *testing.T) {
	e := newCallEnv(t)
	e.nice.Refuse("nadi", 400, "access_denied", "the user is not a member of the organization")
	res, out := e.call(context.Background(), e.caller(orgA, prjP), "listDevices", calldto.Where{}, nil)
	text := wantToolError(t, res, out, "access_denied")
	if !strings.Contains(text, "organization") {
		t.Fatalf("text %q does not explain the reason", text)
	}
	if n := len(e.inv.Received()); n != 0 {
		t.Fatalf("nadi was called %d times", n)
	}
}

func TestCallRefusedWithInvalidScope(t *testing.T) {
	e := newCallEnv(t)
	e.nice.Refuse("nadi", 400, "invalid_scope", "nadi is not active in the project")
	res, out := e.call(context.Background(), e.caller(orgA, prjP), "listDevices", calldto.Where{}, nil)
	wantToolError(t, res, out, "invalid_scope")
	if n := len(e.inv.Received()); n != 0 {
		t.Fatalf("nadi was called %d times", n)
	}
}

func TestCallExplainsAnExpiringUserToken(t *testing.T) {
	e := newCallEnv(t)
	e.nice.Refuse("nadi", 400, "invalid_grant", "subject_token_expiring: the subject token expires within 60 seconds")
	res, out := e.call(context.Background(), e.caller(orgA, prjP), "listDevices", calldto.Where{}, nil)
	text := wantToolError(t, res, out, "subject_token_expiring")
	if !strings.Contains(text, "refresh") {
		t.Fatalf("text %q does not say to refresh the token", text)
	}
}

func TestCallExplainsARefusedAgentClient(t *testing.T) {
	e := newCallEnv(t)
	e.nice.Refuse("nice", 401, "invalid_client", "client authentication failed")
	res, out := e.call(context.Background(), e.caller(orgA, prjP), "listDevices", calldto.Where{}, nil)
	text := wantToolError(t, res, out, "invalid_client")
	if !strings.Contains(text, config.EnvClientSecret) {
		t.Fatalf("text %q does not point the operator to the client", text)
	}
}

func TestCallNamesPermissionDeniedOfTheService(t *testing.T) {
	e := newCallEnv(t)
	e.nice.perms["nadi"] = []string{"nadi.devices.read"}
	res, out := e.call(context.Background(), e.caller(orgA, prjP), "createDevice", calldto.Where{}, newDevice)
	text := wantToolError(t, res, out, "permission_denied")
	if !strings.Contains(text, "nadi") {
		t.Fatalf("text %q does not name the service", text)
	}
}

func TestCallRetriesOnceWhenTheServiceSaysTokenStale(t *testing.T) {
	e := newCallEnv(t)
	e.inv.StaleOnce()
	res, out := e.call(context.Background(), e.caller(orgA, prjP), "createDevice", calldto.Where{}, newDevice)
	got := wantOK(t, res, out)
	if got["name"] != "Sensor" {
		t.Fatalf("device = %v", got)
	}
	if n := len(e.nice.ExchangesFor("nadi")); n != 2 {
		t.Fatalf("exchanges for nadi = %d, want 2 (one fresh exchange after token_stale)", n)
	}
	if n := len(e.inv.Received()); n != 2 {
		t.Fatalf("nadi received %d requests, want 2", n)
	}
}

func TestCallReportsTokenStaleWhenItPersists(t *testing.T) {
	e := newCallEnv(t)
	e.inv.StaleOnce()
	// The second answer is stale too: DoAs returns it, and the tool says so.
	e.nice.onExchange = func(aud string) {
		if aud == "nadi" {
			e.inv.StaleOnce()
		}
	}
	res, out := e.call(context.Background(), e.caller(orgA, prjP), "listDevices", calldto.Where{}, nil)
	wantToolError(t, res, out, "token_stale")
	if n := len(e.inv.Received()); n != 2 {
		t.Fatalf("nadi received %d requests, want 2 (one retry only)", n)
	}
}

func TestCallWhenNICEDoesNotAnswer(t *testing.T) {
	e := newCallEnv(t)
	e.nice.srv.Close()
	res, out := e.call(context.Background(), e.caller(orgA, prjP), "listDevices", calldto.Where{}, nil)
	wantToolError(t, res, out, "nice_unavailable")
}
