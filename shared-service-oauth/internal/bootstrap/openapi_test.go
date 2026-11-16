package bootstrap

import (
	"encoding/json"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/nice-platform/example/shared-service-oauth/docs"
	"github.com/nice-platform/example/shared-service-oauth/internal/adapter/rest"
	restitem "github.com/nice-platform/example/shared-service-oauth/internal/adapter/rest/item"
	"github.com/nice-platform/example/shared-service-oauth/internal/domain"
)

type swaggerDoc struct {
	Swagger string                                `json:"swagger"`
	Paths   map[string]map[string]json.RawMessage `json:"paths"`
	Roles   []struct {
		ID          string   `json:"id"`
		Name        string   `json:"name"`
		Permissions []string `json:"permissions"`
	} `json:"x-nice-roles"`
}

func readDoc(t *testing.T) swaggerDoc {
	t.Helper()
	var d swaggerDoc
	if err := json.Unmarshal(docs.OpenAPI, &d); err != nil {
		t.Fatalf("docs/swagger.json: %v", err)
	}
	if d.Swagger != "2.0" {
		t.Fatalf("swagger = %q, want 2.0", d.Swagger)
	}
	return d
}

// The permission and the sensitive mark NICE imports must be the ones the
// service enforces, and the document must declare nothing else.
func TestOpenAPIMatchesTheRouteTable(t *testing.T) {
	doc := readDoc(t)
	routes := rest.ItemRoutes(nil)
	documented := 0
	for path, item := range doc.Paths {
		for method := range item {
			if method == "parameters" {
				continue
			}
			documented++
			key := strings.ToUpper(method) + " " + path
			if !slices.ContainsFunc(routes, func(r restitem.Route) bool { return r.Method+" "+r.Pattern == key }) {
				t.Errorf("the document declares %s, which the route table does not serve", key)
			}
		}
	}
	if documented != len(routes) {
		t.Errorf("the document declares %d operations, the route table has %d", documented, len(routes))
	}
	for _, rt := range routes {
		key := rt.Method + " " + rt.Pattern
		raw, ok := doc.Paths[rt.Pattern][strings.ToLower(rt.Method)]
		if !ok {
			t.Errorf("%s is served but not in the document", key)
			continue
		}
		var ext map[string]any
		if err := json.Unmarshal(raw, &ext); err != nil {
			t.Fatal(err)
		}
		if got := ext["x-nice-permission"]; got != rt.Permission {
			t.Errorf("%s: x-nice-permission = %v, the service enforces %s", key, got, rt.Permission)
		}
		sensitive, present := ext["x-nice-sensitive"]
		switch {
		case rt.Sensitive && sensitive != true:
			t.Errorf("%s: x-nice-sensitive = %v, the service checks it online", key, sensitive)
		case !rt.Sensitive && present:
			t.Errorf("%s: x-nice-sensitive = %v on an operation that is not sensitive", key, sensitive)
		}
	}
	for _, public := range []string{"/healthz", "/openapi.json", "/docs"} {
		if _, ok := doc.Paths[public]; ok {
			t.Errorf("the document declares %s, which is not an operation of the catalog", public)
		}
	}
}

func TestOpenAPIDeclaresTheOperatorRole(t *testing.T) {
	doc := readDoc(t)
	if len(doc.Roles) != 1 || doc.Roles[0].ID != "operator" || doc.Roles[0].Name == "" {
		t.Fatalf("x-nice-roles = %+v, want one role operator", doc.Roles)
	}
	if got, want := doc.Roles[0].Permissions, []string{domain.PermRead, domain.PermCreate, domain.PermEdit}; !slices.Equal(got, want) {
		t.Fatalf("operator permissions = %v, want %v", got, want)
	}
}

var pathParam = regexp.MustCompile(`^\{.*\}$`)

// The names also follow the default rule of the catalog, so the catalog would
// give the same names if x-nice-permission were removed.
func TestPermissionNamesFollowTheCatalogRule(t *testing.T) {
	action := map[string]string{"GET": "read", "POST": "create", "PUT": "edit", "PATCH": "edit", "DELETE": "delete"}
	for _, rt := range rest.ItemRoutes(nil) {
		var segs []string
		for s := range strings.SplitSeq(strings.Trim(rt.Pattern, "/"), "/") {
			if !pathParam.MatchString(s) {
				segs = append(segs, s)
			}
		}
		if want := domain.ServiceSlug + "." + strings.Join(segs, ".") + "." + action[rt.Method]; rt.Permission != want {
			t.Errorf("%s %s: permission %s, the catalog rule gives %s", rt.Method, rt.Pattern, rt.Permission, want)
		}
	}
}
