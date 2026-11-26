package bootstrap

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	calldto "github.com/nice-platform/example/agent/backend/internal/app/call/dto"
	chatdto "github.com/nice-platform/example/agent/backend/internal/app/chat/dto"
)

// chatToolsFor runs the chat tools with the production call wiring, for the user of a token in orgA/prjP.
func chatToolsFor(t *testing.T, e *testEnv) (func(name string, in map[string]any, ap *calldto.Approval) chatdto.ToolOutcome, calldto.Caller) {
	t.Helper()
	calls, err := NewCallService(e.cfg, http.DefaultClient, nil)
	if err != nil {
		t.Fatal(err)
	}
	tools := NewChatTools(calls)
	tok := e.userToken(orgA, prjP)
	claims := readClaims(t, tok)
	c := calldto.Caller{Token: tok, Subject: claims["sub"].(string), OrgID: orgA, ProjectID: prjP}
	return func(name string, in map[string]any, ap *calldto.Approval) chatdto.ToolOutcome {
		b, _ := json.Marshal(in)
		return tools.Run(context.Background(), c, chatdto.ToolCall{ID: "c1", Name: name, Input: b}, ap)
	}, c
}

func TestChatToolsRunTheSameUseCasesAsMCP(t *testing.T) {
	e := newTestEnv(t)
	run, _ := chatToolsFor(t, e)

	out := run("list_services", nil, nil)
	if out.IsError || !strings.Contains(string(out.Content), `"nadi"`) {
		t.Fatalf("list_services: %s", out.Content)
	}
	out = run("list_operations", map[string]any{"service": "nadi"}, nil)
	if out.IsError || !strings.Contains(string(out.Content), "listDevices") {
		t.Fatalf("list_operations: %s", out.Content)
	}
	out = run("call_operation", op("createDevice", map[string]any{"body": map[string]any{"name": "Sensor", "serial_number": "SN-1", "status": "active"}}), nil)
	if out.IsError || !strings.Contains(string(out.Content), `"status":201`) {
		t.Fatalf("create: %s", out.Content)
	}
	out = run("call_operation", op("listDevices", nil), nil)
	if out.IsError || !strings.Contains(string(out.Content), "Sensor") {
		t.Fatalf("list: %s", out.Content)
	}
	for _, o := range []chatdto.ToolOutcome{out} {
		if strings.Contains(string(o.Content), "eyJ") {
			t.Fatalf("a token is in a tool result: %s", o.Content)
		}
	}
}

func TestChatToolsReportFailuresAsTheMCPToolsDo(t *testing.T) {
	e := newTestEnv(t)
	run, _ := chatToolsFor(t, e)
	out := run("list_operations", map[string]any{"service": "pengiriman"}, nil)
	if !out.IsError || !strings.Contains(string(out.Content), `"error":"service_not_allowed"`) {
		t.Fatalf("pengiriman: %s", out.Content)
	}
	out = run("call_operation", map[string]any{"service": "nadi", "operation": "listDevices", "extra": 1}, nil)
	if !out.IsError || !strings.Contains(string(out.Content), "invalid_argument") {
		t.Fatalf("unknown field: %s", out.Content)
	}
	out = run("rm_rf", nil, nil)
	if !out.IsError || !strings.Contains(string(out.Content), "unknown tool") {
		t.Fatalf("unknown tool: %s", out.Content)
	}
}

func TestChatToolsAskBeforeASensitiveOperation(t *testing.T) {
	e := newTestEnv(t)
	run, _ := chatToolsFor(t, e)
	if out := run("call_operation", op("createDevice", map[string]any{"body": map[string]any{"name": "Sensor", "serial_number": "SN-1", "status": "active"}}), nil); out.IsError {
		t.Fatalf("create: %s", out.Content)
	}
	before := len(e.nice.ExchangesFor("nadi"))
	ask := run("call_operation", deleteDev1, nil)
	if ask.Approval == nil || !strings.Contains(ask.Approval.Message, "Delete") || deletes(e) != 0 {
		t.Fatalf("first run: %+v, deletes %d", ask, deletes(e))
	}
	no := run("call_operation", deleteDev1, &calldto.Approval{State: ask.Approval.State, Accepted: false})
	if !no.IsError || !strings.Contains(string(no.Content), "not_approved") || deletes(e) != 0 {
		t.Fatalf("declined: %s, deletes %d", no.Content, deletes(e))
	}
	if after := len(e.nice.ExchangesFor("nadi")); after != before {
		t.Fatalf("a token for nadi was exchanged without approval: %d -> %d", before, after)
	}
	ask = run("call_operation", deleteDev1, nil)
	yes := run("call_operation", deleteDev1, &calldto.Approval{State: ask.Approval.State, Accepted: true})
	if yes.IsError || deletes(e) != 1 {
		t.Fatalf("approved: %s, deletes %d", yes.Content, deletes(e))
	}
	again := run("call_operation", deleteDev1, &calldto.Approval{State: ask.Approval.State, Accepted: true})
	if again.Approval == nil || deletes(e) != 1 {
		t.Fatalf("reused state: %+v, deletes %d", again, deletes(e))
	}
}
