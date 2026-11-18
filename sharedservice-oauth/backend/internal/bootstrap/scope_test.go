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
	r := e.do(http.MethodPost, "/devices", tokenIn(t, orgA, projectP, allPerms...), map[string]any{"name": "Milik P", "serial_number": "SN-1", "status": "active"})
	if r.Status != http.StatusCreated {
		t.Fatalf("create in P = %d %s", r.Status, r.Body)
	}
	id := r.JSON(t)["id"].(string)

	q := tokenIn(t, orgA, projectQ, allPerms...)
	// The project comes from prj only, so a client that names P is still Q.
	r = e.do(http.MethodGet, "/devices?project_id="+projectP, q, nil, "X-Nice-Project", projectP)
	if r.Status != http.StatusOK || strings.TrimSpace(r.Body) != `{"devices":[]}` {
		t.Fatalf("list of Q = %d %s, want an empty list", r.Status, r.Body)
	}
	for _, c := range []struct {
		method string
		body   any
	}{
		{http.MethodGet, nil},
		{http.MethodPut, map[string]any{"name": "Diambil Q", "serial_number": "SN-1", "status": "active"}},
		{http.MethodDelete, nil},
	} {
		if r = e.do(c.method, "/devices/"+id, q, c.body); r.Status != http.StatusNotFound {
			t.Errorf("%s of the device of P from Q = %d %s, want 404", c.method, r.Status, r.Body)
		}
	}
	r = e.do(http.MethodGet, "/devices/"+id, tokenIn(t, orgA, projectP, domain.PermRead), nil)
	if r.Status != http.StatusOK || r.JSON(t)["name"] != "Milik P" {
		t.Fatalf("device of P after Q tried = %d %s", r.Status, r.Body)
	}
}

func TestProjectFromTheClientIsIgnored(t *testing.T) {
	e := newTestEnv(t, nil)
	p := tokenIn(t, orgA, projectP, allPerms...)
	r := e.do(http.MethodPost, "/devices?project_id="+projectQ, p, map[string]any{"name": "Sensor", "serial_number": "SN-1", "status": "active"})
	if r.Status != http.StatusCreated {
		t.Fatalf("create = %d %s", r.Status, r.Body)
	}
	id := r.JSON(t)["id"].(string)
	if r = e.do(http.MethodGet, "/devices/"+id, p, nil); r.Status != http.StatusOK {
		t.Fatalf("the device is not in the project of the token: %d %s", r.Status, r.Body)
	}
	if r = e.do(http.MethodGet, "/devices", tokenIn(t, orgA, projectQ, domain.PermRead), nil); strings.Contains(r.Body, id) {
		t.Fatalf("the query moved the device to %s: %s", projectQ, r.Body)
	}

	// A body never carries the scope: an unknown field is refused like any other.
	r = e.do(http.MethodPost, "/devices", p, map[string]any{"name": "Sensor", "serial_number": "SN-1", "status": "active", "project_id": projectQ})
	wantError(t, r, http.StatusBadRequest, "invalid_argument", "body")
	if devices, err := e.store.List(context.Background(), scopeOf(orgA, projectP)); err != nil || len(devices) != 1 {
		t.Fatalf("devices of P = %+v, %v; want only the first", devices, err)
	}
}

func TestTokenWithoutProjectIsRefused(t *testing.T) {
	e := newTestEnv(t, &fakeChecker{allowed: true})
	it := e.create(orgA, "Sensor")
	noPrj := tokenIn(t, orgA, "", allPerms...)
	for _, c := range []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, "/devices", nil},
		{http.MethodPost, "/devices", map[string]any{"name": "Baru", "serial_number": "SN-1", "status": "active"}},
		{http.MethodPost, "/devices", `not JSON`},
		{http.MethodGet, "/devices/" + it.ID, nil},
		{http.MethodPut, "/devices/" + it.ID, map[string]any{"name": "Baru", "serial_number": "SN-1", "status": "active"}},
		{http.MethodDelete, "/devices/" + it.ID, nil},
	} {
		t.Run(c.method+" "+c.path, func(t *testing.T) {
			wantError(t, e.do(c.method, c.path, noPrj, c.body), http.StatusForbidden, "insufficient_scope", "project_required")
		})
	}
	if devices := e.list(orgA); len(devices) != 1 || devices[0].Name != "Sensor" {
		t.Fatalf("devices after the refusals = %+v", devices)
	}
}

func TestPermissionIsCheckedBeforeTheProject(t *testing.T) {
	e := newTestEnv(t, nil)
	wantError(t, e.do(http.MethodPost, "/devices", tokenIn(t, orgA, "", domain.PermRead), map[string]any{"name": "x", "serial_number": "SN-1", "status": "active"}),
		http.StatusForbidden, "insufficient_scope", "permission_denied")
}
