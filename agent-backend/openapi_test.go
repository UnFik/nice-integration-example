package main

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
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
	if err := json.Unmarshal(openAPIDoc, &d); err != nil {
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
	if !slices.Equal(ops, []string{"POST " + mcpPath}) {
		t.Fatalf("operations = %v, want only POST %s", ops, mcpPath)
	}
	var op map[string]any
	if err := json.Unmarshal(doc.Paths[mcpPath]["post"], &op); err != nil {
		t.Fatal(err)
	}
	if op["x-nice-permission"] != permUse {
		t.Fatalf("x-nice-permission = %v, the agent enforces %s", op["x-nice-permission"], permUse)
	}
	if _, ok := op["x-nice-sensitive"]; ok {
		t.Fatalf("POST %s is marked sensitive; the agent does not check it online", mcpPath)
	}
	if op["operationId"] == nil {
		t.Fatalf("POST %s has no operationId", mcpPath)
	}
}

func TestOpenAPIDeclaresAUserRole(t *testing.T) {
	doc := readOpenAPI(t)
	if len(doc.Roles) != 1 || doc.Roles[0].ID != "user" || !slices.Equal(doc.Roles[0].Permissions, []string{permUse}) {
		t.Fatalf("x-nice-roles = %+v, want one role user with %s", doc.Roles, permUse)
	}
}

func TestPermissionNamesStartWithTheSlug(t *testing.T) {
	if serviceSlug != "agen" || !strings.HasPrefix(permUse, serviceSlug+".") {
		t.Fatalf("slug %q, permission %q", serviceSlug, permUse)
	}
}
