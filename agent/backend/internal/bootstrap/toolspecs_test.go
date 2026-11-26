package bootstrap

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	mcpadapter "github.com/nice-platform/example/agent/backend/internal/adapter/mcp"
)

// The chat model must see the tools exactly as an MCP client does.
func TestToolSpecsEqualTheMCPToolList(t *testing.T) {
	e := newTestEnv(t)
	res, err := e.connect(e.userToken(orgA, prjP)).ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	listed := map[string]struct {
		desc   string
		schema any
	}{}
	for _, tool := range res.Tools {
		raw, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatal(err)
		}
		var schema any
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatal(err)
		}
		listed[tool.Name] = struct {
			desc   string
			schema any
		}{tool.Description, schema}
	}

	specs := mcpadapter.ToolSpecs()
	if len(specs) != len(listed) {
		t.Fatalf("ToolSpecs has %d tools, tools/list %d", len(specs), len(listed))
	}
	for _, s := range specs {
		l, ok := listed[s.Name]
		if !ok {
			t.Errorf("ToolSpecs has %s, tools/list does not", s.Name)
			continue
		}
		if s.Description != l.desc {
			t.Errorf("%s: description %q, tools/list %q", s.Name, s.Description, l.desc)
		}
		var schema any
		if err := json.Unmarshal(s.InputSchema, &schema); err != nil {
			t.Fatalf("%s: input schema is not JSON: %v", s.Name, err)
		}
		if !reflect.DeepEqual(schema, l.schema) {
			t.Errorf("%s: input schema %s differs from tools/list %v", s.Name, s.InputSchema, l.schema)
		}
	}
}
