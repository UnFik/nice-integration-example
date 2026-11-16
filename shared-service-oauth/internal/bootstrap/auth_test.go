package bootstrap

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/nice-platform/example/shared-service-oauth/docs"
	"github.com/nice-platform/example/shared-service-oauth/internal/domain"
)

func TestMissingToken(t *testing.T) {
	e := newTestEnv(t, nil)
	r := e.do(http.MethodGet, "/items", "", nil)
	wantError(t, r, http.StatusUnauthorized, "invalid_token", "missing_token")
	if got := r.Header.Get("WWW-Authenticate"); got != "Bearer" {
		t.Fatalf("WWW-Authenticate = %q, want Bearer", got)
	}
}

func TestTokenRefusals(t *testing.T) {
	e := newTestEnv(t, nil)
	now := time.Now()
	for _, c := range []struct {
		name   string
		edit   func(map[string]any)
		reason string
	}{
		{"token for another service", func(c map[string]any) { c["aud"] = "pengiriman" }, "wrong_audience"},
		{"token for two services", func(c map[string]any) { c["aud"] = []string{"inventaris", "pengiriman"} }, "wrong_audience"},
		{"expired beyond the skew", func(c map[string]any) { c["exp"] = now.Add(-61 * time.Second).Unix() }, "expired"},
		{"another issuer", func(c map[string]any) { c["iss"] = "http://evil.test" }, "wrong_issuer"},
	} {
		t.Run(c.name, func(t *testing.T) {
			cl := claims(orgA, domain.PermRead)
			c.edit(cl)
			r := e.do(http.MethodGet, "/items", sign(t, cl), nil)
			wantError(t, r, http.StatusUnauthorized, "invalid_token", c.reason)
			if got := r.Header.Get("WWW-Authenticate"); !strings.HasPrefix(got, "Bearer ") || !strings.Contains(got, c.reason) {
				t.Fatalf("WWW-Authenticate = %q", got)
			}
		})
	}
}

func TestExpiryAllowsTheClockSkew(t *testing.T) {
	e := newTestEnv(t, nil)
	cl := claims(orgA, domain.PermRead)
	cl["exp"] = time.Now().Add(-30 * time.Second).Unix()
	if r := e.do(http.MethodGet, "/items", sign(t, cl), nil); r.Status != http.StatusOK {
		t.Fatalf("a token expired 30 seconds ago = %d %s, want 200 (skew 60 seconds)", r.Status, r.Body)
	}
}

func TestPermissionIsEnforcedPerOperation(t *testing.T) {
	e := newTestEnv(t, nil)
	reader := token(t, orgA, domain.PermRead)
	if r := e.do(http.MethodGet, "/items", reader, nil); r.Status != http.StatusOK {
		t.Fatalf("read with %s = %d %s", domain.PermRead, r.Status, r.Body)
	}
	r := e.do(http.MethodPost, "/items", reader, map[string]any{"name": "Kabel", "quantity": 1})
	wantError(t, r, http.StatusForbidden, "insufficient_scope", "permission_denied")
	if items := e.store.List(orgA); len(items) != 0 {
		t.Fatalf("a refused request created %v", items)
	}
	it := e.create(orgA, "Kabel")
	wantError(t, e.do(http.MethodPut, "/items/"+it.ID, reader, map[string]any{"name": "Baru", "quantity": 1}),
		http.StatusForbidden, "insufficient_scope", "permission_denied")
	wantError(t, e.do(http.MethodGet, "/items", token(t, orgA, domain.PermCreate), nil),
		http.StatusForbidden, "insufficient_scope", "permission_denied")
	if got, _ := e.store.Get(orgA, it.ID); got.Name != "Kabel" {
		t.Fatalf("a refused request renamed the item to %q", got.Name)
	}
}

func TestOrganizationsDoNotSeeEachOther(t *testing.T) {
	e := newTestEnv(t, nil)
	r := e.do(http.MethodPost, "/items", token(t, orgA, allPerms...), map[string]any{"name": "Milik A", "quantity": 1})
	if r.Status != http.StatusCreated {
		t.Fatalf("create = %d %s", r.Status, r.Body)
	}
	id := r.JSON(t)["id"].(string)

	b := token(t, orgB, allPerms...)
	// The tenant comes from nice_org_id only, so a client that names org A is still org B.
	r = e.do(http.MethodGet, "/items?organization_id="+orgA, b, nil, "X-Nice-Organization", orgA)
	if r.Status != http.StatusOK || strings.TrimSpace(r.Body) != `{"items":[]}` {
		t.Fatalf("list of org B = %d %s, want an empty list", r.Status, r.Body)
	}
	if r = e.do(http.MethodGet, "/items/"+id, b, nil); r.Status != http.StatusNotFound {
		t.Fatalf("org B reading the item of org A = %d, want 404", r.Status)
	}
	if r = e.do(http.MethodPut, "/items/"+id, b, map[string]any{"name": "Diambil B", "quantity": 1}); r.Status != http.StatusNotFound {
		t.Fatalf("org B editing the item of org A = %d, want 404", r.Status)
	}
	if got, ok := e.store.Get(orgA, id); !ok || got.Name != "Milik A" {
		t.Fatalf("item of org A = %+v %v", got, ok)
	}
}

func TestFirstRequestOfAnOrganizationCreatesItsTenant(t *testing.T) {
	e := newTestEnv(t, nil)
	if e.store.HasTenant(orgB) {
		t.Fatal("tenant exists before any request")
	}
	r := e.do(http.MethodGet, "/items", token(t, orgB, domain.PermRead), nil)
	if r.Status != http.StatusOK || strings.TrimSpace(r.Body) != `{"items":[]}` {
		t.Fatalf("first list = %d %s, want 200 with an empty list", r.Status, r.Body)
	}
	if !e.store.HasTenant(orgB) {
		t.Fatal("the first request did not create the tenant")
	}
}

func TestPreflightFromTheRegisteredOrigin(t *testing.T) {
	e := newTestEnv(t, nil)
	r := e.do(http.MethodOptions, "/items/itm_1", "", nil,
		"Origin", testOrigin, "Access-Control-Request-Method", "DELETE", "Access-Control-Request-Headers", "authorization")
	if r.Status != http.StatusNoContent {
		t.Fatalf("preflight = %d %s, want 204", r.Status, r.Body)
	}
	h := r.Header
	if h.Get("Access-Control-Allow-Origin") != testOrigin {
		t.Fatalf("Access-Control-Allow-Origin = %q", h.Get("Access-Control-Allow-Origin"))
	}
	for _, want := range []string{"GET", "POST", "PUT", "DELETE"} {
		if !strings.Contains(h.Get("Access-Control-Allow-Methods"), want) {
			t.Errorf("Access-Control-Allow-Methods = %q, missing %s", h.Get("Access-Control-Allow-Methods"), want)
		}
	}
	for _, want := range []string{"Authorization", "Content-Type"} {
		if !strings.Contains(h.Get("Access-Control-Allow-Headers"), want) {
			t.Errorf("Access-Control-Allow-Headers = %q, missing %s", h.Get("Access-Control-Allow-Headers"), want)
		}
	}
}

func TestPreflightFromAnotherOrigin(t *testing.T) {
	e := newTestEnv(t, nil)
	r := e.do(http.MethodOptions, "/items", "", nil,
		"Origin", "http://evil.test", "Access-Control-Request-Method", "GET", "Access-Control-Request-Headers", "authorization")
	if v := r.Header.Get("Access-Control-Allow-Origin"); v != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q for an origin that is not registered", v)
	}
	r = e.do(http.MethodGet, "/items", token(t, orgA, domain.PermRead), nil, "Origin", "http://evil.test")
	if v := r.Header.Get("Access-Control-Allow-Origin"); v != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q on a request from an origin that is not registered", v)
	}
}

func TestCORSHeadersReachErrorsSoTheBrowserCanReadThem(t *testing.T) {
	e := newTestEnv(t, nil)
	r := e.do(http.MethodGet, "/items", "", nil, "Origin", testOrigin)
	if r.Status != http.StatusUnauthorized || r.Header.Get("Access-Control-Allow-Origin") != testOrigin {
		t.Fatalf("401 from the registered origin = %d, Access-Control-Allow-Origin %q", r.Status, r.Header.Get("Access-Control-Allow-Origin"))
	}
}

func TestHealthzNeedsNoToken(t *testing.T) {
	e := newTestEnv(t, nil)
	if r := e.do(http.MethodGet, "/healthz", "", nil); r.Status != http.StatusOK {
		t.Fatalf("healthz = %d %s", r.Status, r.Body)
	}
}

func TestOpenAPINeedsNoToken(t *testing.T) {
	e := newTestEnv(t, nil)
	r := e.do(http.MethodGet, "/openapi.json", "", nil)
	if r.Status != http.StatusOK || !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		t.Fatalf("openapi.json = %d %q", r.Status, r.Header.Get("Content-Type"))
	}
	if !bytes.Equal([]byte(r.Body), docs.OpenAPI) {
		t.Fatal("/openapi.json is not the embedded document")
	}
}
