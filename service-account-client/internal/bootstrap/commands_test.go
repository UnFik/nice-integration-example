package bootstrap

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/nice-platform/example/service-account-client/internal/platform/config"
)

func TestListProvesTheKeyToNICEAndCallsNadiWithTheToken(t *testing.T) {
	w := newWorld(t)
	if code := w.run(context.Background(), nil, "list"); code != 0 {
		t.Fatalf("exit %d: %s", code, w.err)
	}
	reqs := w.nice.Requests()
	if len(reqs) != 1 || reqs[0].Get("grant_type") != "client_credentials" || reqs[0].Get("client_id") != testAccount ||
		reqs[0].Get("client_secret") != testKey || reqs[0].Get("scope") != "nice:aud:nadi" {
		t.Fatalf("token requests = %d", len(reqs))
	}
	calls := w.nadi.Calls()
	if len(calls) != 1 || calls[0].Method != "GET" || calls[0].Path != "/devices" || calls[0].Bearer() != w.nice.Issued()[0] {
		t.Fatalf("calls to nadi = %d", len(calls))
	}
	if out := w.out.String(); !strings.Contains(out, "dev_1") || !strings.Contains(out, "SN-2026-0001") || !strings.Contains(out, "CREATED BY") {
		t.Fatalf("stdout = %s", out)
	}
}

func TestCreateSendsTheFieldsAndPrintsTheID(t *testing.T) {
	w := newWorld(t)
	code := w.run(context.Background(), nil, "create", "-name", "Sensor Pintu", "-serial", "SN-2026-0002", "-status", "maintenance", "-location", "Gudang B")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, w.err)
	}
	calls := w.nadi.Calls()
	if len(calls) != 1 || calls[0].Method != "POST" || calls[0].Path != "/devices" || calls[0].Header.Get("Content-Type") != "application/json" {
		t.Fatalf("calls to nadi = %+v", calls)
	}
	var sent map[string]string
	if err := json.Unmarshal([]byte(calls[0].Body), &sent); err != nil {
		t.Fatal(err)
	}
	if sent["name"] != "Sensor Pintu" || sent["serial_number"] != "SN-2026-0002" || sent["status"] != "maintenance" || sent["location"] != "Gudang B" {
		t.Fatalf("sent = %v", sent)
	}
	if strings.TrimSpace(w.out.String()) != "dev_2" {
		t.Fatalf("stdout = %q", w.out)
	}
}

func TestAStaleTokenIsRenewedAndTheRequestSentOnceMore(t *testing.T) {
	w := newWorld(t)
	w.nadi.Script(refusal{401, "invalid_token", "token_stale"})
	if code := w.run(context.Background(), nil, "list"); code != 0 {
		t.Fatalf("exit %d: %s", code, w.err)
	}
	calls, issued := w.nadi.Calls(), w.nice.Issued()
	if len(issued) != 2 || len(calls) != 2 || calls[0].Bearer() != issued[0] || calls[1].Bearer() != issued[1] {
		t.Fatalf("tokens = %d, calls = %d", len(issued), len(calls))
	}
	if !strings.Contains(w.out.String(), "dev_1") {
		t.Fatalf("stdout = %s", w.out)
	}
}

func TestAStaleTokenIsNotTriedAThirdTime(t *testing.T) {
	w := newWorld(t)
	w.nadi.Script(refusal{401, "invalid_token", "token_stale"}, refusal{401, "invalid_token", "token_stale"})
	if code := w.run(context.Background(), nil, "list"); code != 1 {
		t.Fatalf("exit %d", code)
	}
	if n := len(w.nadi.Calls()); n != 2 {
		t.Fatalf("calls to nadi = %d, want 2", n)
	}
	if !strings.Contains(w.err.String(), "token_stale") {
		t.Fatalf("stderr = %s", w.err)
	}
}

func TestAMissingPermissionIsExplainedAndNotRepeated(t *testing.T) {
	w := newWorld(t)
	w.nadi.Script(refusal{403, "insufficient_scope", "permission_denied"})
	if code := w.run(context.Background(), nil, "create", "-name", "Sensor", "-serial", "SN-9"); code != 1 {
		t.Fatalf("exit %d", code)
	}
	if n := len(w.nadi.Calls()); n != 1 {
		t.Fatalf("calls to nadi = %d, want 1", n)
	}
	for _, want := range []string{"permission_denied", "nadi.devices.create", "checklist step 3"} {
		if !strings.Contains(w.err.String(), want) {
			t.Errorf("stderr lacks %q: %s", want, w.err)
		}
	}
	if w.out.String() != "" {
		t.Errorf("stdout = %s", w.out)
	}
}

func TestFieldsNadiRefusesArePrintedWithItsCode(t *testing.T) {
	w := newWorld(t)
	w.nadi.Script(refusal{400, "invalid_argument", "serial_number"})
	if code := w.run(context.Background(), nil, "create", "-name", "Sensor", "-serial", "SN-9"); code != 1 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(w.err.String(), "invalid_argument (serial_number)") {
		t.Fatalf("stderr = %s", w.err)
	}
}

func TestARefusedKeyIsExplained(t *testing.T) {
	w := newWorld(t)
	w.env["NICE_SA_API_KEY"] = "nice_sk_wrong.key"
	if code := w.run(context.Background(), nil, "list"); code != 1 {
		t.Fatalf("exit %d", code)
	}
	if n := len(w.nadi.Calls()); n != 0 {
		t.Fatalf("nadi was called %d times with no token", n)
	}
	for _, want := range []string{"invalid_client", "revoked", "checklist step 4"} {
		if !strings.Contains(w.err.String(), want) {
			t.Errorf("stderr lacks %q: %s", want, w.err)
		}
	}
	if strings.Contains(w.err.String(), "nice_sk_wrong.key") {
		t.Error("the refused key is in stderr")
	}
}

func TestARefusedScopeIsExplained(t *testing.T) {
	w := newWorld(t)
	w.nice.Refuse(&refusal{400, "invalid_scope", "the service is not active in the project"})
	if code := w.run(context.Background(), nil, "list"); code != 1 {
		t.Fatalf("exit %d", code)
	}
	for _, want := range []string{"invalid_scope", "not entitled", "checklist step 2"} {
		if !strings.Contains(w.err.String(), want) {
			t.Errorf("stderr lacks %q: %s", want, w.err)
		}
	}
	if n := len(w.nadi.Calls()); n != 0 {
		t.Fatalf("nadi was called %d times with no token", n)
	}
}

func TestTheSideThatDidNotAnswerIsNamed(t *testing.T) {
	w := newWorld(t)
	w.niceNet.down.Store(true)
	if code := w.run(context.Background(), nil, "list"); code != 1 || !strings.Contains(w.err.String(), "nice_unavailable") {
		t.Fatalf("NICE down: exit %d: %s", code, w.err)
	}
	w = newWorld(t)
	w.nadiNet.down.Store(true)
	if code := w.run(context.Background(), nil, "list"); code != 1 || !strings.Contains(w.err.String(), "service_unavailable") {
		t.Fatalf("nadi down: exit %d: %s", code, w.err)
	}
}

func TestAnUnknownCommandPrintsTheUsageBeforeAnySetting(t *testing.T) {
	w := newWorld(t)
	w.env = nil
	for _, args := range [][]string{nil, {"delete"}} {
		if code := w.run(context.Background(), nil, args...); code != 2 || !strings.Contains(w.err.String(), "usage: service-account-client") {
			t.Fatalf("%v: exit %d: %s", args, code, w.err)
		}
	}
	if len(w.nice.Requests()) != 0 {
		t.Fatal("NICE was called")
	}
}

func TestAMissingSettingStopsBeforeAnyCall(t *testing.T) {
	w := newWorld(t)
	delete(w.env, "NICE_SA_API_KEY")
	if code := w.run(context.Background(), nil, "list"); code != 1 || !strings.Contains(w.err.String(), "NICE_SA_API_KEY is required") {
		t.Fatalf("exit %d: %s", code, w.err)
	}
	if len(w.nice.Requests()) != 0 || len(w.nadi.Calls()) != 0 {
		t.Fatal("a call was made without the key")
	}
}

// The SDK sends the token over https only, loopback excepted: a plain http address of nadi stops the command before
// the key is proved to NICE.
func TestANadiAddressInTheClearIsRefusedBeforeNICEIsCalled(t *testing.T) {
	w := newWorld(t)
	w.env[config.EnvNadiURL] = "http://nadi.example.com"
	if code := w.run(context.Background(), nil, "list"); code != 1 {
		t.Fatalf("exit %d, want 1: %s", code, w.err)
	}
	if !strings.Contains(w.err.String(), "FAILED:") || !strings.Contains(w.err.String(), "nadi.example.com") {
		t.Fatalf("stderr names the address: %s", w.err)
	}
	if len(w.nice.Requests()) != 0 || len(w.nadi.Calls()) != 0 {
		t.Fatalf("nothing is called: %d token requests, %d calls to nadi", len(w.nice.Requests()), len(w.nadi.Calls()))
	}
}
