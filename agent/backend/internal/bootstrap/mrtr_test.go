package bootstrap

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The approval of a sensitive operation rests on three behaviours of the MCP Go SDK in the stateless JSON mode of
// agen: a tool result with InputRequests reaches a 2026-07-28 client as input_required, the retry brings
// inputResponses and requestState back to the handler, and the session reports the protocol version of the request.

type mrtrProbe struct {
	mu       sync.Mutex
	versions []string
	states   []string
	answers  []mcp.InputResponseMap
}

func (p *mrtrProbe) handler(_ context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	version := ""
	if ip := req.Session.InitializeParams(); ip != nil {
		version = ip.ProtocolVersion
	}
	p.mu.Lock()
	p.versions = append(p.versions, version)
	p.states = append(p.states, req.Params.RequestState)
	p.answers = append(p.answers, req.Params.InputResponses)
	p.mu.Unlock()
	if req.Params.RequestState == "" {
		return &mcp.CallToolResult{
			InputRequests: mcp.InputRequestMap{"approve": &mcp.ElicitParams{
				Mode: "form", Message: "approve?",
				RequestedSchema: map[string]any{"type": "object", "properties": map[string]any{"approve": map[string]any{"type": "boolean"}}},
			}},
			RequestState: "state-1",
		}, nil, nil
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "done"}}}, nil, nil
}

func newMRTRServer(t *testing.T, p *mrtrProbe) *httptest.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "probe", Version: "1"}, nil)
	mcp.AddTool(s, &mcp.Tool{Name: "probe"}, p.handler)
	h := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func TestMCPSDKMultiRoundTripInStatelessJSONMode(t *testing.T) {
	p := &mrtrProbe{}
	srv := newMRTRServer(t, p)
	var asked []string
	c := mcp.NewClient(&mcp.Implementation{Name: "c", Version: "1"}, &mcp.ClientOptions{
		ElicitationHandler: func(_ context.Context, r *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			asked = append(asked, r.Params.Message)
			return &mcp.ElicitResult{Action: "accept", Content: map[string]any{"approve": true}}, nil
		},
	})
	s, err := c.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: srv.URL}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	res, err := s.CallTool(context.Background(), &mcp.CallToolParams{Name: "probe"})
	if err != nil {
		t.Fatal(err)
	}
	if len(asked) != 1 || asked[0] != "approve?" {
		t.Fatalf("elicitations = %v, want one", asked)
	}
	if txt, _ := res.Content[0].(*mcp.TextContent); txt == nil || txt.Text != "done" {
		t.Fatalf("result = %+v", res)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.states) != 2 || p.states[1] != "state-1" {
		t.Fatalf("states = %v, want the retry to echo state-1", p.states)
	}
	got, ok := p.answers[1]["approve"].(*mcp.ElicitResult)
	if !ok || got.Action != "accept" || got.Content["approve"] != true {
		t.Fatalf("input responses of the retry = %#v", p.answers[1])
	}
	for _, v := range p.versions {
		if v != "2026-07-28" {
			t.Fatalf("protocol versions seen by the handler = %v", p.versions)
		}
	}
}

// An old client in stateless mode cannot be asked: the handler sees its version, so agen refuses before asking.
func TestMCPSDKOldProtocolVersionIsVisibleInStatelessMode(t *testing.T) {
	p := &mrtrProbe{}
	srv := newMRTRServer(t, p)
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"probe","arguments":{}}}`
	req, _ := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2025-06-18")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.versions) == 0 || p.versions[0] != "2025-06-18" {
		t.Fatalf("versions = %v, answer %s", p.versions, raw)
	}
}
