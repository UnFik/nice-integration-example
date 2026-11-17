package bootstrap

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

const (
	projectP = "prj_pppppppp"
	projectQ = "prj_qqqqqqqq"
)

// tokenIn is a user token of org for project; an empty project leaves prj out.
func tokenIn(t *testing.T, org, project string, perms ...string) string {
	t.Helper()
	c := claims(org, perms...)
	if project == "" {
		delete(c, "prj")
	} else {
		c["prj"] = project
	}
	return sign(t, c)
}

func TestProjectsOfAnOrganizationDoNotSeeEachOther(t *testing.T) {
	e := newTestEnv(t, &fakeChecker{allowed: true})
	r := e.do(http.MethodPost, "/items", tokenIn(t, orgA, projectP, allPerms...), map[string]any{"name": "Milik P", "quantity": 1})
	if r.Status != http.StatusCreated {
		t.Fatalf("create in P = %d %s", r.Status, r.Body)
	}
	id := r.JSON(t)["id"].(string)

	q := tokenIn(t, orgA, projectQ, allPerms...)
	// The project comes from prj only, so a client that names P is still Q.
	r = e.do(http.MethodGet, "/items?project_id="+projectP, q, nil, "X-Nice-Project", projectP)
	if r.Status != http.StatusOK || strings.TrimSpace(r.Body) != `{"items":[]}` {
		t.Fatalf("list of Q = %d %s, want an empty list", r.Status, r.Body)
	}
	for _, c := range []struct {
		method string
		body   any
	}{
		{http.MethodGet, nil},
		{http.MethodPut, map[string]any{"name": "Diambil Q", "quantity": 1}},
		{http.MethodDelete, nil},
	} {
		if r = e.do(c.method, "/items/"+id, q, c.body); r.Status != http.StatusNotFound {
			t.Errorf("%s of the item of P from Q = %d %s, want 404", c.method, r.Status, r.Body)
		}
	}
	r = e.do(http.MethodGet, "/items/"+id, tokenIn(t, orgA, projectP, domain.PermRead), nil)
	if r.Status != http.StatusOK || r.JSON(t)["name"] != "Milik P" {
		t.Fatalf("item of P after Q tried = %d %s", r.Status, r.Body)
	}
}

func TestProjectFromTheClientIsIgnored(t *testing.T) {
	e := newTestEnv(t, nil)
	p := tokenIn(t, orgA, projectP, allPerms...)
	r := e.do(http.MethodPost, "/items?project_id="+projectQ, p, map[string]any{"name": "Kabel", "quantity": 1})
	if r.Status != http.StatusCreated {
		t.Fatalf("create = %d %s", r.Status, r.Body)
	}
	id := r.JSON(t)["id"].(string)
	if r = e.do(http.MethodGet, "/items/"+id, p, nil); r.Status != http.StatusOK {
		t.Fatalf("the item is not in the project of the token: %d %s", r.Status, r.Body)
	}
	if r = e.do(http.MethodGet, "/items", tokenIn(t, orgA, projectQ, domain.PermRead), nil); strings.Contains(r.Body, id) {
		t.Fatalf("the query moved the item to %s: %s", projectQ, r.Body)
	}

	// A body never carries the scope: an unknown field is refused like any other.
	r = e.do(http.MethodPost, "/items", p, map[string]any{"name": "Kabel", "quantity": 1, "project_id": projectQ})
	wantError(t, r, http.StatusBadRequest, "invalid_argument", "body")
	if items, err := e.store.List(context.Background(), scopeOf(orgA, projectP)); err != nil || len(items) != 1 {
		t.Fatalf("items of P = %+v, %v; want only the first", items, err)
	}
}

func TestTokenWithoutProjectIsRefused(t *testing.T) {
	e := newTestEnv(t, &fakeChecker{allowed: true})
	it := e.create(orgA, "Kabel")
	noPrj := tokenIn(t, orgA, "", allPerms...)
	for _, c := range []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, "/items", nil},
		{http.MethodPost, "/items", map[string]any{"name": "Baru", "quantity": 1}},
		{http.MethodPost, "/items", `not JSON`},
		{http.MethodGet, "/items/" + it.ID, nil},
		{http.MethodPut, "/items/" + it.ID, map[string]any{"name": "Baru", "quantity": 1}},
		{http.MethodDelete, "/items/" + it.ID, nil},
	} {
		t.Run(c.method+" "+c.path, func(t *testing.T) {
			wantError(t, e.do(c.method, c.path, noPrj, c.body), http.StatusForbidden, "insufficient_scope", "project_required")
		})
	}
	if items := e.list(orgA); len(items) != 1 || items[0].Name != "Kabel" {
		t.Fatalf("items after the refusals = %+v", items)
	}
}

func TestPermissionIsCheckedBeforeTheProject(t *testing.T) {
	e := newTestEnv(t, nil)
	wantError(t, e.do(http.MethodPost, "/items", tokenIn(t, orgA, "", domain.PermRead), map[string]any{"name": "x", "quantity": 1}),
		http.StatusForbidden, "insufficient_scope", "permission_denied")
}
