package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	mcpadapter "github.com/nice-platform/example/agent/backend/internal/adapter/mcp"
	"github.com/nice-platform/example/agent/backend/internal/domain"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// noPassthrough fails when a service ever received the user's own token (aud agen) instead of an exchanged one.
func noPassthrough(t *testing.T, e *testEnv) {
	t.Helper()
	for _, s := range []*fakeService{e.inv} {
		for _, c := range s.Received() {
			aud, _ := c["aud"].([]any)
			if c["aud"] == domain.ServiceSlug || (len(aud) > 0 && aud[0] == domain.ServiceSlug) {
				t.Fatalf("%s received the user's token for agen", s.slug)
			}
			if act, _ := c["act"].(map[string]any); act["sub"] != testClientID {
				t.Fatalf("%s received a token without the agent as actor: %v", s.slug, c)
			}
		}
	}
}

func callTool(t *testing.T, s *mcp.ClientSession, name string, args map[string]any) (*mcp.CallToolResult, map[string]any) {
	t.Helper()
	res, err := s.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if res.IsError {
		return res, nil
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("%s: structured content %s", name, raw)
	}
	return res, out
}

// op is the arguments of call_operation for an operation of nadi.
func op(key string, extra map[string]any) map[string]any {
	args := map[string]any{"service": "nadi", "operation": key}
	for k, v := range extra {
		args[k] = v
	}
	return args
}

// body is the JSON body nadi answered, from the result of call_operation.
func body(t *testing.T, out map[string]any) map[string]any {
	t.Helper()
	b, ok := out["body"].(map[string]any)
	if !ok {
		t.Fatalf("call_operation answered %v, want a status and a JSON object body", out)
	}
	return b
}

func TestToolListHasTheFourDiscoveryTools(t *testing.T) {
	e := newTestEnv(t)
	res, err := e.connect(e.userToken(orgA, prjP)).ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
		a := tool.Annotations
		switch tool.Name {
		case "list_services", "list_operations":
			if a == nil || !a.ReadOnlyHint {
				t.Errorf("tool %s must be annotated read-only", tool.Name)
			}
		case "get_operation_schema":
			if a == nil || !a.ReadOnlyHint {
				t.Errorf("tool %s must be annotated read-only", tool.Name)
			}
			raw, _ := json.Marshal(tool.InputSchema)
			var schema struct {
				Properties map[string]struct {
					Type  any `json:"type"`
					Items struct {
						Type any `json:"type"`
					} `json:"items"`
				} `json:"properties"`
				Required []string `json:"required"`
			}
			_ = json.Unmarshal(raw, &schema)
			ops := schema.Properties["operations"]
			if fmt.Sprint(ops.Type) != "array" || fmt.Sprint(ops.Items.Type) != "string" ||
				!slices.Contains(schema.Required, "service") || !slices.Contains(schema.Required, "operations") {
				t.Errorf("get_operation_schema: operations must be a required array of strings; schema %s", raw)
			}
		case "call_operation":
			if a == nil || a.ReadOnlyHint || a.DestructiveHint == nil || !*a.DestructiveHint {
				t.Errorf("call_operation can delete: it must be annotated destructive")
			}
			raw, _ := json.Marshal(tool.InputSchema)
			var schema struct {
				Properties map[string]any `json:"properties"`
				Required   []string       `json:"required"`
			}
			_ = json.Unmarshal(raw, &schema)
			for _, arg := range []string{"organization_id", "project_id", "path_params", "query", "body"} {
				if _, ok := schema.Properties[arg]; !ok || slices.Contains(schema.Required, arg) {
					t.Errorf("call_operation: %s must be an optional argument; schema %s", arg, raw)
				}
			}
			for _, arg := range []string{"service", "operation"} {
				if !slices.Contains(schema.Required, arg) {
					t.Errorf("call_operation: %s must be required; schema %s", arg, raw)
				}
			}
		}
	}
	slices.Sort(names)
	if want := []string{"call_operation", "get_operation_schema", "list_operations", "list_services"}; !slices.Equal(names, want) {
		t.Fatalf("tools = %v, want %v", names, want)
	}
}

func TestListServicesShowsTheMapWithoutAddresses(t *testing.T) {
	e := newTestEnv(t)
	res, out := callTool(t, e.connect(e.userToken(orgA, prjP)), "list_services", nil)
	raw, _ := json.Marshal(out)
	if res.IsError || strings.Contains(string(raw), e.inv.srv.URL) {
		t.Fatalf("list_services = %s", raw)
	}
	orgs, _ := out["organizations"].([]any)
	if len(orgs) != 2 || !strings.Contains(string(raw), `"services":["nadi"]`) || !strings.Contains(string(raw), "Lokasi Q") {
		t.Fatalf("list_services = %s", raw)
	}
	if n := len(e.nice.ExchangesFor("nice")); n != 1 || len(e.nice.ExchangesFor("nadi")) != 0 {
		t.Fatalf("exchanges = %+v, want one for the NICE API only", e.nice.Exchanges())
	}
}

func TestListOperationsMarksTheSensitiveOne(t *testing.T) {
	e := newTestEnv(t)
	_, out := callTool(t, e.connect(e.userToken(orgA, prjP)), "list_operations", map[string]any{"service": "nadi"})
	ops, _ := out["operations"].([]any)
	sensitive := map[string]bool{}
	for _, o := range ops {
		m := o.(map[string]any)
		sensitive[m["key"].(string)] = m["sensitive"] == true
	}
	if len(ops) != 4 || !sensitive["deleteDevice"] || sensitive["listDevices"] || sensitive["createDevice"] {
		t.Fatalf("operations = %v", ops)
	}
}

func TestListOperationsOfAServiceTheUserMayNotCall(t *testing.T) {
	e := newTestEnv(t)
	res, _ := callTool(t, e.connect(e.userToken(orgA, prjP)), "list_operations", map[string]any{"service": "pengiriman"})
	wantToolError(t, res, nil, "service_not_allowed")
}

func TestDiscoveryOnANICEWithoutIt(t *testing.T) {
	e := newTestEnv(t)
	e.nice.noDiscovery = true
	res, _ := callTool(t, e.connect(e.userToken(orgA, prjP)), "list_services", nil)
	text := wantToolError(t, res, nil, "discovery_unavailable")
	if !strings.Contains(text, "upgrade NICE") {
		t.Fatalf("text %q does not say to upgrade NICE", text)
	}
}

func TestCallOperationInTheContextOfTheUserToken(t *testing.T) {
	e := newTestEnv(t)
	s := e.connect(e.userToken(orgA, prjP))

	_, created := callTool(t, s, "call_operation", op("createDevice",
		map[string]any{"body": map[string]any{"name": "Sensor Suhu", "serial_number": "SN-1", "status": "active"}}))
	device := body(t, created)
	if created["status"] != float64(201) || device["name"] != "Sensor Suhu" || device["created_by"] != testUser || device["acted_by"] != testClientID {
		t.Fatalf("created = %v, want 201 with created_by the user and acted_by the agent's client", created)
	}
	_, listed := callTool(t, s, "call_operation", op("listDevices", map[string]any{"query": map[string]any{"status": "active"}}))
	devices := body(t, listed)
	if list, _ := devices["devices"].([]any); len(list) != 1 || devices["org"] != orgA || devices["project"] != prjP || devices["status"] != "active" {
		t.Fatalf("devices = %v", devices)
	}
	for _, ex := range e.nice.ExchangesFor("nadi") {
		if strings.Contains(ex.Scope, "nice:org") || strings.Contains(ex.Scope, "nice:prj") {
			t.Fatalf("exchange %q names a context; without arguments the token's context is NICE's default", ex.Scope)
		}
	}
	if n := len(e.nice.ExchangesFor("nadi")); n != 1 {
		t.Fatalf("exchanges for nadi = %d, want 1 (then reused)", n)
	}
	noPassthrough(t, e)
}

func TestCallOperationInAnotherOrganizationOfTheUser(t *testing.T) {
	e := newTestEnv(t)
	_, out := callTool(t, e.connect(e.userToken(orgA, prjP)), "call_operation",
		op("listDevices", map[string]any{"organization_id": orgB, "project_id": prjQ}))
	if devices := body(t, out); devices["org"] != orgB || devices["project"] != prjQ {
		t.Fatalf("devices = %v, want the context %s/%s", devices, orgB, prjQ)
	}
	ex := e.nice.ExchangesFor("nadi")
	if len(ex) != 1 || ex[0].Scope != "nice:aud:nadi nice:org:"+orgB+" nice:prj:"+prjQ {
		t.Fatalf("exchanges = %+v", ex)
	}
	noPassthrough(t, e)
}

func TestCallOperationOutsideTheMap(t *testing.T) {
	e := newTestEnv(t)
	res, _ := callTool(t, e.connect(e.userToken(orgA, prjP)), "call_operation",
		op("listDevices", map[string]any{"organization_id": orgB, "project_id": prjP}))
	wantToolError(t, res, nil, "invalid_argument")
	if n := len(e.nice.ExchangesFor("nadi")); n != 0 {
		t.Fatalf("exchanges for nadi = %d, want none", n)
	}
}

func TestCallOperationRefusedByNICEWithAccessDenied(t *testing.T) {
	e := newTestEnv(t)
	e.nice.Refuse("nadi", 400, "access_denied", "the user is not a member of the organization")
	res, _ := callTool(t, e.connect(e.userToken(orgA, prjP)), "call_operation", op("listDevices", nil))
	wantToolError(t, res, nil, "access_denied")
	if n := len(e.inv.Received()); n != 0 {
		t.Fatalf("nadi was called %d times", n)
	}
}

func TestCallOperationNeedsAProjectInAnotherOrganization(t *testing.T) {
	e := newTestEnv(t)
	res, _ := callTool(t, e.connect(e.userToken(orgA, prjP)), "call_operation", op("listDevices", map[string]any{"organization_id": orgB}))
	wantToolError(t, res, nil, "project_id")
	if n := len(e.nice.ExchangesFor("nadi")); n != 0 {
		t.Fatalf("exchanges for nadi = %d, want none", n)
	}
}

func TestCallOperationRefusesArgumentsThatWouldMoveIt(t *testing.T) {
	e := newTestEnv(t)
	s := e.connect(e.userToken(orgA, prjP))
	for _, args := range []map[string]any{
		op("listDevices", map[string]any{"organization_id": orgB + " nice:aud:nice", "project_id": prjQ}),
		op("listDevices", map[string]any{"project_id": prjQ + " nice:org:" + orgC}),
		{"service": "nadi nice:aud:nice", "operation": "listDevices"},
		op("getDevice", map[string]any{"path_params": map[string]any{"id": "../devices"}}),
		op("getDevice", map[string]any{"path_params": map[string]any{"id": "a/b"}}),
		op("getDevice", nil),
		op("getDevice", map[string]any{"path_params": map[string]any{"id": "dev-1", "other": "x"}}),
		op("listDevices", map[string]any{"query": map[string]any{"limit": "5"}}),
		op("rebootDevice", nil),
	} {
		res, _ := callTool(t, s, "call_operation", args)
		wantToolError(t, res, nil, "invalid_argument")
	}
	if n := len(e.nice.ExchangesFor("nadi")); n != 0 || len(e.inv.Requests()) != 0 {
		t.Fatalf("exchanges for nadi = %d, nadi requests %v, want none", n, e.inv.Requests())
	}
}

func TestCallOperationOfAServiceOutsideTheMap(t *testing.T) {
	e := newTestEnv(t)
	res, _ := callTool(t, e.connect(e.userToken(orgA, prjP)), "call_operation",
		map[string]any{"service": "pengiriman", "operation": "listShipments"})
	wantToolError(t, res, nil, "service_not_allowed")
}

func TestCallOperationOnAPlainHTTPCatalogEndpoint(t *testing.T) {
	e := newTestEnv(t)
	e.nice.endpoint = "http://nadi.internal:8081"
	res, _ := callTool(t, e.connect(e.userToken(orgA, prjP)), "call_operation", op("listDevices", nil))
	wantToolError(t, res, nil, "insecure_endpoint")
	if n := len(e.nice.ExchangesFor("nadi")); n != 0 {
		t.Fatalf("exchanges for nadi = %d, want none", n)
	}
}

func TestCallOperationWithoutThePermissionInTheService(t *testing.T) {
	e := newTestEnv(t)
	e.nice.perms["nadi"] = []string{"nadi.devices.read"}
	res, _ := callTool(t, e.connect(e.userToken(orgA, prjP)), "call_operation",
		op("createDevice", map[string]any{"body": map[string]any{"name": "Sensor", "serial_number": "SN-2", "status": "active"}}))
	wantToolError(t, res, nil, "permission_denied")
}

// The README calls a tool with one curl request: no session, no initialize, a plain JSON answer.
func TestToolCallInOneHTTPRequest(t *testing.T) {
	e := newTestEnv(t)
	resp := e.post(mcpadapter.MCPPath, e.userToken(orgA, prjP),
		`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"call_operation","arguments":{"service":"nadi","operation":"listDevices"}}}`)
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
		t.Fatalf("status %d, content type %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	var rpc struct {
		ID     int                `json:"id"`
		Result mcp.CallToolResult `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpc); err != nil {
		t.Fatal(err)
	}
	if rpc.ID != 7 || rpc.Result.IsError || rpc.Result.StructuredContent == nil {
		t.Fatalf("answer = %+v", rpc)
	}
}
