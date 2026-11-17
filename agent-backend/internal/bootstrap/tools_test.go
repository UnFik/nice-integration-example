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
	for _, s := range []*fakeService{e.inv, e.nice.pengiriman} {
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

func TestToolListHasTheFourToolsAndNothingSensitive(t *testing.T) {
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
	want := []string{"inventaris_create_item", "inventaris_list_items", "pengiriman_create_shipment", "pengiriman_list_shipments"}
	if !slices.Equal(names, want) {
		t.Fatalf("tools = %v, want %v", names, want)
	}
}

func TestEveryToolInTheContextOfTheUserToken(t *testing.T) {
	e := newTestEnv(t)
	s := e.connect(e.userToken(orgA, prjP))

	_, item := callTool(t, s, "inventaris_create_item", map[string]any{"name": "Kabel UTP", "quantity": 3})
	if item == nil || item["name"] != "Kabel UTP" || item["created_by"] != testUser || item["acted_by"] != testClientID {
		t.Fatalf("created item = %v, want created_by the user and acted_by the agent's client", item)
	}
	_, items := callTool(t, s, "inventaris_list_items", nil)
	if list, _ := items["items"].([]any); len(list) != 1 || items["org"] != orgA || items["project"] != prjP {
		t.Fatalf("items = %v", items)
	}
	_, shipment := callTool(t, s, "pengiriman_create_shipment", map[string]any{"destination": "Gudang Timur, Surabaya"})
	if shipment == nil || shipment["destination"] != "Gudang Timur, Surabaya" {
		t.Fatalf("shipment = %v", shipment)
	}
	_, shipments := callTool(t, s, "pengiriman_list_shipments", map[string]any{})
	if list, _ := shipments["shipments"].([]any); len(list) != 1 {
		t.Fatalf("shipments = %v", shipments)
	}
	for _, ex := range e.nice.Exchanges() {
		if strings.Contains(ex.Scope, "nice:org") || strings.Contains(ex.Scope, "nice:prj") {
			t.Fatalf("exchange %q names a context; without arguments the token's context is NICE's default", ex.Scope)
		}
	}
	if n := len(e.nice.Exchanges()); n != 2 {
		t.Fatalf("exchanges = %d, want 2 (one per service, then reused)", n)
	}
	noPassthrough(t, e)
}

func TestToolInAnotherOrganizationOfTheUser(t *testing.T) {
	e := newTestEnv(t)
	s := e.connect(e.userToken(orgA, prjP))
	_, items := callTool(t, s, "inventaris_list_items", map[string]any{"organization_id": orgB, "project_id": prjQ})
	if items == nil || items["org"] != orgB || items["project"] != prjQ {
		t.Fatalf("items = %v, want the context %s/%s", items, orgB, prjQ)
	}
	ex := e.nice.Exchanges()
	if len(ex) != 1 || ex[0].Scope != "nice:aud:inventaris nice:org:"+orgB+" nice:prj:"+prjQ {
		t.Fatalf("exchanges = %+v", ex)
	}
	noPassthrough(t, e)
}

func TestToolInAnOrganizationThatIsNotTheUsers(t *testing.T) {
	e := newTestEnv(t)
	s := e.connect(e.userToken(orgA, prjP))
	res, _ := callTool(t, s, "pengiriman_list_shipments", map[string]any{"organization_id": orgC, "project_id": prjQ})
	wantToolError(t, res, nil, "access_denied")
	if n := len(e.nice.pengiriman.Received()); n != 0 {
		t.Fatalf("pengiriman was called %d times", n)
	}
}

func TestToolNeedsAProjectInAnotherOrganization(t *testing.T) {
	e := newTestEnv(t)
	s := e.connect(e.userToken(orgA, prjP))
	res, _ := callTool(t, s, "inventaris_list_items", map[string]any{"organization_id": orgB})
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
		res, _ := callTool(t, s, "inventaris_list_items", args)
		wantToolError(t, res, nil, "invalid_argument")
	}
	if n := len(e.nice.Exchanges()); n != 0 {
		t.Fatalf("exchanges = %d, want none", n)
	}
}

func TestToolWithoutThePermissionInTheService(t *testing.T) {
	e := newTestEnv(t)
	e.nice.perms["inventaris"] = []string{"inventaris.items.read"}
	s := e.connect(e.userToken(orgA, prjP))
	res, _ := callTool(t, s, "inventaris_create_item", map[string]any{"name": "Kabel", "quantity": 1})
	wantToolError(t, res, nil, "permission_denied")
}

// The README calls a tool with one curl request: no session, no initialize, a plain JSON answer.
func TestToolCallInOneHTTPRequest(t *testing.T) {
	e := newTestEnv(t)
	resp := e.post(mcpadapter.MCPPath, e.userToken(orgA, prjP),
		`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"inventaris_list_items","arguments":{}}}`)
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
