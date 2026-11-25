package bootstrap

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	mcpadapter "github.com/nice-platform/example/agent-backend/internal/adapter/mcp"
	"github.com/nice-platform/example/agent-backend/internal/domain"

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

func TestToolListHasTheTwoNadiToolsAndNothingSensitive(t *testing.T) {
	e := newTestEnv(t)
	res, err := e.connect(e.userToken(orgA, prjP)).ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
		if strings.Contains(tool.Name, "delete") || strings.Contains(tool.Name, "cancel") {
			t.Errorf("tool %s does a sensitive operation", tool.Name)
		}
		raw, _ := json.Marshal(tool.InputSchema)
		var schema struct {
			Properties map[string]any `json:"properties"`
			Required   []string       `json:"required"`
		}
		_ = json.Unmarshal(raw, &schema)
		for _, arg := range []string{"organization_id", "project_id"} {
			if _, ok := schema.Properties[arg]; !ok || slices.Contains(schema.Required, arg) {
				t.Errorf("tool %s: %s must be an optional argument; schema %s", tool.Name, arg, raw)
			}
		}
		if tool.Annotations == nil || tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint {
			t.Errorf("tool %s must be annotated as not destructive", tool.Name)
		}
	}
	slices.Sort(names)
	want := []string{"nadi_create_device", "nadi_list_devices"}
	if !slices.Equal(names, want) {
		t.Fatalf("tools = %v, want %v", names, want)
	}
}

func TestToolListHasNoTargetThatIsNotNadi(t *testing.T) {
	e := newTestEnv(t)
	res, err := e.connect(e.userToken(orgA, prjP)).ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range res.Tools {
		if !strings.HasPrefix(tool.Name, string(domain.Nadi)+"_") {
			t.Errorf("tool %s targets something other than nadi", tool.Name)
		}
	}
}

func TestEveryToolInTheContextOfTheUserToken(t *testing.T) {
	e := newTestEnv(t)
	s := e.connect(e.userToken(orgA, prjP))

	_, device := callTool(t, s, "nadi_create_device", map[string]any{"name": "Sensor Suhu", "serial_number": "SN-1", "status": "active"})
	if device == nil || device["name"] != "Sensor Suhu" || device["created_by"] != testUser || device["acted_by"] != testClientID {
		t.Fatalf("created device = %v, want created_by the user and acted_by the agent's client", device)
	}
	_, devices := callTool(t, s, "nadi_list_devices", nil)
	if list, _ := devices["devices"].([]any); len(list) != 1 || devices["org"] != orgA || devices["project"] != prjP {
		t.Fatalf("devices = %v", devices)
	}
	for _, ex := range e.nice.Exchanges() {
		if strings.Contains(ex.Scope, "nice:org") || strings.Contains(ex.Scope, "nice:prj") {
			t.Fatalf("exchange %q names a context; without arguments the token's context is NICE's default", ex.Scope)
		}
	}
	if n := len(e.nice.Exchanges()); n != 1 {
		t.Fatalf("exchanges = %d, want 1 (one for nadi, then reused)", n)
	}
	noPassthrough(t, e)
}

func TestToolInAnotherOrganizationOfTheUser(t *testing.T) {
	e := newTestEnv(t)
	s := e.connect(e.userToken(orgA, prjP))
	_, devices := callTool(t, s, "nadi_list_devices", map[string]any{"organization_id": orgB, "project_id": prjQ})
	if devices == nil || devices["org"] != orgB || devices["project"] != prjQ {
		t.Fatalf("devices = %v, want the context %s/%s", devices, orgB, prjQ)
	}
	ex := e.nice.Exchanges()
	if len(ex) != 1 || ex[0].Scope != "nice:aud:nadi nice:org:"+orgB+" nice:prj:"+prjQ {
		t.Fatalf("exchanges = %+v", ex)
	}
	noPassthrough(t, e)
}

func TestToolInAnOrganizationThatIsNotTheUsers(t *testing.T) {
	e := newTestEnv(t)
	s := e.connect(e.userToken(orgA, prjP))
	res, _ := callTool(t, s, "nadi_list_devices", map[string]any{"organization_id": orgC, "project_id": prjQ})
	wantToolError(t, res, nil, "access_denied")
	if n := len(e.inv.Received()); n != 0 {
		t.Fatalf("nadi was called %d times", n)
	}
}

func TestToolNeedsAProjectInAnotherOrganization(t *testing.T) {
	e := newTestEnv(t)
	s := e.connect(e.userToken(orgA, prjP))
	res, _ := callTool(t, s, "nadi_list_devices", map[string]any{"organization_id": orgB})
	wantToolError(t, res, nil, "project_id")
	if n := len(e.nice.Exchanges()); n != 0 {
		t.Fatalf("exchanges = %d, want none", n)
	}
}

func TestToolRefusesIDsThatWouldAddScopes(t *testing.T) {
	e := newTestEnv(t)
	s := e.connect(e.userToken(orgA, prjP))
	for _, args := range []map[string]any{
		{"organization_id": orgB + " nice:aud:nice", "project_id": prjQ},
		{"project_id": prjQ + " nice:org:" + orgC},
	} {
		res, _ := callTool(t, s, "nadi_list_devices", args)
		wantToolError(t, res, nil, "invalid_argument")
	}
	if n := len(e.nice.Exchanges()); n != 0 {
		t.Fatalf("exchanges = %d, want none", n)
	}
}

func TestToolWithoutThePermissionInTheService(t *testing.T) {
	e := newTestEnv(t)
	e.nice.perms["nadi"] = []string{"nadi.devices.read"}
	s := e.connect(e.userToken(orgA, prjP))
	res, _ := callTool(t, s, "nadi_create_device", map[string]any{"name": "Sensor", "serial_number": "SN-2", "status": "active"})
	wantToolError(t, res, nil, "permission_denied")
}

func TestCreateDeviceSendsLocationOnlyWhenGiven(t *testing.T) {
	e := newTestEnv(t)
	s := e.connect(e.userToken(orgA, prjP))
	_, without := callTool(t, s, "nadi_create_device", map[string]any{"name": "Sensor", "serial_number": "SN-1", "status": "active"})
	if _, ok := without["location"]; without == nil || ok {
		t.Fatalf("device = %v, want no location sent", without)
	}
	_, with := callTool(t, s, "nadi_create_device",
		map[string]any{"name": "Sensor", "serial_number": "SN-2", "status": "maintenance", "location": "Gudang A"})
	if with == nil || with["location"] != "Gudang A" || with["status"] != "maintenance" || with["serial_number"] != "SN-2" {
		t.Fatalf("device = %v", with)
	}
}

func TestCreateDeviceRefusesAStatusNadiDoesNotKnow(t *testing.T) {
	e := newTestEnv(t)
	s := e.connect(e.userToken(orgA, prjP))
	res, err := s.CallTool(context.Background(), &mcp.CallToolParams{Name: "nadi_create_device",
		Arguments: map[string]any{"name": "Sensor", "serial_number": "SN-1", "status": "broken"}})
	if err == nil && !res.IsError {
		t.Fatalf("status broken was accepted: %+v", res)
	}
	if n := len(e.nice.Exchanges()); n != 0 {
		t.Fatalf("exchanges = %d, want none", n)
	}
}

// The README calls a tool with one curl request: no session, no initialize, a plain JSON answer.
func TestToolCallInOneHTTPRequest(t *testing.T) {
	e := newTestEnv(t)
	resp := e.post(mcpadapter.MCPPath, e.userToken(orgA, prjP),
		`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"nadi_list_devices","arguments":{}}}`)
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
