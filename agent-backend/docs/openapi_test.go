package docs_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/nice-platform/example/agent-backend/docs"
	mcpadapter "github.com/nice-platform/example/agent-backend/internal/adapter/mcp"
	"github.com/nice-platform/example/agent-backend/internal/domain"
)

type openAPIDocument struct {
	OpenAPI string                                `json:"openapi"`
	Info    struct{ Title string }                `json:"info"`
	Paths   map[string]map[string]json.RawMessage `json:"paths"`
	Roles   []struct {
		ID          string   `json:"id"`
		Name        string   `json:"name"`
		Permissions []string `json:"permissions"`
	} `json:"x-nice-roles"`
}

func readOpenAPI(t *testing.T) openAPIDocument {
	t.Helper()
	var d openAPIDocument
	if err := json.Unmarshal(docs.OpenAPI, &d); err != nil {
		t.Fatalf("openapi.json: %v", err)
	}
	if !strings.HasPrefix(d.OpenAPI, "3.0.") {
		t.Fatalf("openapi = %q, want 3.0.x", d.OpenAPI)
	}
	return d
}

// NICE imports this document into its catalog: the one operation must carry the permission the agent enforces on
// /mcp, and nothing else may be declared.
func TestOpenAPIDeclaresOnlyTheMCPEndpointWithItsPermission(t *testing.T) {
	doc := readOpenAPI(t)
	var ops []string
	for path, item := range doc.Paths {
		for method := range item {
			if method != "parameters" {
				ops = append(ops, strings.ToUpper(method)+" "+path)
			}
		}
	}
	if !slices.Equal(ops, []string{"POST " + mcpadapter.MCPPath}) {
		t.Fatalf("operations = %v, want only POST %s", ops, mcpadapter.MCPPath)
	}
	var op map[string]any
	if err := json.Unmarshal(doc.Paths[mcpadapter.MCPPath]["post"], &op); err != nil {
		t.Fatal(err)
	}
	if op["x-nice-permission"] != domain.PermUse {
		t.Fatalf("x-nice-permission = %v, the agent enforces %s", op["x-nice-permission"], domain.PermUse)
	}
	if _, ok := op["x-nice-sensitive"]; ok {
		t.Fatalf("POST %s is marked sensitive; the agent does not check it online", mcpadapter.MCPPath)
	}
	if op["operationId"] == nil {
		t.Fatalf("POST %s has no operationId", mcpadapter.MCPPath)
	}
}

func TestOpenAPIDeclaresAUserRole(t *testing.T) {
	doc := readOpenAPI(t)
	if len(doc.Roles) != 1 || doc.Roles[0].ID != "user" || !slices.Equal(doc.Roles[0].Permissions, []string{domain.PermUse}) {
		t.Fatalf("x-nice-roles = %+v, want one role user with %s", doc.Roles, domain.PermUse)
	}
}

func TestPermissionNamesStartWithTheSlug(t *testing.T) {
	if domain.ServiceSlug != "agen" || !strings.HasPrefix(domain.PermUse, domain.ServiceSlug+".") {
		t.Fatalf("slug %q, permission %q", domain.ServiceSlug, domain.PermUse)
	}
}
