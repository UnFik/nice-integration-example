package bootstrap

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"slices"
	"strings"
	"sync"
	"testing"

	mcpadapter "github.com/nice-platform/example/agent/backend/internal/adapter/mcp"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// user plays the user behind an MCP client: it records every approval asked and answers with action and approve.
type user struct {
	mu      sync.Mutex
	asked   []string
	action  string
	approve bool
}

func (u *user) handler(_ context.Context, r *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.asked = append(u.asked, r.Params.Message)
	if u.action != "accept" {
		return &mcp.ElicitResult{Action: u.action}, nil
	}
	return &mcp.ElicitResult{Action: "accept", Content: map[string]any{"approve": u.approve}}, nil
}

func deletes(e *testEnv) int {
	return len(slices.DeleteFunc(e.inv.Requests(), func(r string) bool { return !strings.HasPrefix(r, "DELETE ") }))
}

// withDevice creates dev-1 in nadi so a delete has something to delete.
func withDevice(t *testing.T, e *testEnv, s *mcp.ClientSession) {
	t.Helper()
	res, _ := callTool(t, s, "call_operation", op("createDevice",
		map[string]any{"body": map[string]any{"name": "Sensor", "serial_number": "SN-1", "status": "active"}}))
	if res.IsError {
		t.Fatalf("create: %s", resultText(t, res))
	}
}

var deleteDev1 = op("deleteDevice", map[string]any{"path_params": map[string]any{"id": "dev-1"}})

func TestSensitiveOperationIsSentOnceAfterTheUserApproves(t *testing.T) {
	e := newTestEnv(t)
	u := &user{action: "accept", approve: true}
	s := e.connectWith(e.userToken(orgA, prjP), &mcp.ClientOptions{ElicitationHandler: u.handler})
	withDevice(t, e, s)

	res, out := callTool(t, s, "call_operation", deleteDev1)
	if res.IsError || out["status"] != float64(204) {
		t.Fatalf("delete = %v %s", out, resultText(t, res))
	}
	if len(u.asked) != 1 {
		t.Fatalf("approvals asked = %v, want one", u.asked)
	}
	for _, want := range []string{"NADI", "Delete a device", "DELETE /devices/dev-1", "Org A", "Demo"} {
		if !strings.Contains(u.asked[0], want) {
			t.Errorf("approval %q does not name %q", u.asked[0], want)
		}
	}
	if n := deletes(e); n != 1 {
		t.Fatalf("nadi received %d deletes, want 1", n)
	}
	noPassthrough(t, e)
}

func TestSensitiveOperationDeclinedOrDismissedSendsNothing(t *testing.T) {
	for _, u := range []*user{{action: "decline"}, {action: "cancel"}, {action: "accept", approve: false}} {
		e := newTestEnv(t)
		s := e.connectWith(e.userToken(orgA, prjP), &mcp.ClientOptions{ElicitationHandler: u.handler})
		res, _ := callTool(t, s, "call_operation", deleteDev1)
		wantToolError(t, res, nil, "not_approved")
		if len(u.asked) != 1 || len(e.nice.ExchangesFor("nadi")) != 0 || len(e.inv.Requests()) != 0 {
			t.Fatalf("%+v: asked %d, exchanges for nadi %d, nadi requests %v, want one question and nothing else",
				u, len(u.asked), len(e.nice.ExchangesFor("nadi")), e.inv.Requests())
		}
	}
}

func TestNonSensitiveOperationAsksNothing(t *testing.T) {
	e := newTestEnv(t)
	u := &user{action: "accept", approve: true}
	s := e.connectWith(e.userToken(orgA, prjP), &mcp.ClientOptions{ElicitationHandler: u.handler})
	callTool(t, s, "call_operation", op("listDevices", nil))
	withDevice(t, e, s)
	if len(u.asked) != 0 {
		t.Fatalf("approvals asked = %v, want none", u.asked)
	}
}

func TestSensitiveOperationInAClientThatCannotAsk(t *testing.T) {
	e := newTestEnv(t)
	// A 2026-07-28 client without elicitation.
	res, _ := callTool(t, e.connect(e.userToken(orgA, prjP)), "call_operation", deleteDev1)
	wantToolError(t, res, nil, "approval_unavailable")

	// A client on an older protocol: the stateless server cannot send it an elicitation.
	resp := e.post(mcpadapter.MCPPath, e.userToken(orgA, prjP), rpcCall(1, "", deleteDev1, ""))
	raw, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(raw), "approval_unavailable") {
		t.Fatalf("old client answer = %s", raw)
	}
	if n := len(e.nice.ExchangesFor("nadi")); n != 0 || len(e.inv.Requests()) != 0 {
		t.Fatalf("exchanges for nadi = %d, nadi requests %v, want none", n, e.inv.Requests())
	}
}

// rpcCall is a tools/call of call_operation as JSON-RPC. With version 2026-07-28 the request carries the _meta of the
// sessionless protocol, with form elicitation; state and approve make it the retry after an approval.
func rpcCall(id int, version string, args map[string]any, state string, approve ...bool) string {
	params := map[string]any{"name": "call_operation", "arguments": args}
	if version != "" {
		params["_meta"] = map[string]any{
			mcp.MetaKeyProtocolVersion:    version,
			mcp.MetaKeyClientInfo:         map[string]any{"name": "curl", "version": "1"},
			mcp.MetaKeyClientCapabilities: map[string]any{"elicitation": map[string]any{"form": map[string]any{}}},
		}
	}
	if state != "" {
		params["requestState"] = state
		params["inputResponses"] = map[string]any{"approve": map[string]any{"action": "accept", "content": map[string]any{"approve": approve[0]}}}
	}
	raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": "tools/call", "params": params})
	return string(raw)
}

type rpcResult struct {
	Result struct {
		ResultType    string          `json:"resultType"`
		RequestState  string          `json:"requestState"`
		InputRequests json.RawMessage `json:"inputRequests"`
		IsError       bool            `json:"isError"`
		Content       []struct {
			Text string `json:"text"`
		} `json:"content"`
	} `json:"result"`
	Error json.RawMessage `json:"error"`
}

func (e *testEnv) rpc(token, body string) rpcResult {
	e.t.Helper()
	// The headers of protocol 2026-07-28 repeat the version, the method and the tool of the body.
	req := e.postHeaders(mcpadapter.MCPPath, token, body,
		map[string]string{"MCP-Protocol-Version": "2026-07-28", "Mcp-Method": "tools/call", "Mcp-Name": "call_operation"})
	var r rpcResult
	raw, _ := io.ReadAll(req.Body)
	if err := json.Unmarshal(raw, &r); err != nil {
		e.t.Fatalf("answer %s: %v", raw, err)
	}
	return r
}

// The two rounds the README shows with curl, and a state that must not approve anything else.
func TestApprovalStateBindsTheUserAndTheCall(t *testing.T) {
	e := newTestEnv(t)
	tok := e.userToken(orgA, prjP)
	withDevice(t, e, e.connect(tok))
	const v = "2026-07-28"

	first := e.rpc(tok, rpcCall(1, v, deleteDev1, ""))
	if first.Result.ResultType != "input_required" || first.Result.RequestState == "" || !strings.Contains(string(first.Result.InputRequests), "elicitation/create") {
		t.Fatalf("first round = %+v %s", first.Result, first.Error)
	}
	state := first.Result.RequestState
	if raw, _ := base64.RawURLEncoding.DecodeString(strings.Split(state, ".")[0]); strings.Contains(state, testUser) || strings.Contains(string(raw), testUser) {
		t.Fatalf("state %q shows the user", state)
	}

	deleteDev2 := op("deleteDevice", map[string]any{"path_params": map[string]any{"id": "dev-2"}})
	// The same user with a token of the next refresh: the approval follows the user, not the token string.
	other := e.userToken(orgA, prjP)
	otherUser := sign(t, func() map[string]any {
		c := userClaims(e.issuer, "agen", orgA, prjP, "agen.mcp.use")
		c["sub"] = "usr_other"
		return c
	}())
	for name, retry := range map[string]rpcResult{
		"another call":  e.rpc(tok, rpcCall(2, v, deleteDev2, state, true)),
		"another user":  e.rpc(otherUser, rpcCall(3, v, deleteDev1, state, true)),
		"altered state": e.rpc(tok, rpcCall(4, v, deleteDev1, state+"x", true)),
	} {
		if retry.Result.ResultType != "input_required" {
			t.Errorf("%s: retry = %+v, want a new approval asked", name, retry.Result)
		}
	}
	if n := deletes(e); n != 0 {
		t.Fatalf("nadi received %d deletes before the approval", n)
	}

	done := e.rpc(other, rpcCall(5, v, deleteDev1, state, true))
	if done.Result.IsError || done.Result.ResultType == "input_required" {
		t.Fatalf("approved retry = %+v %s", done.Result, done.Error)
	}
	again := e.rpc(tok, rpcCall(6, v, deleteDev1, state, true))
	if again.Result.ResultType != "input_required" {
		t.Fatalf("replayed state = %+v, want a new approval asked", again.Result)
	}
	if n := deletes(e); n != 1 {
		t.Fatalf("nadi received %d deletes, want exactly 1", n)
	}
}
