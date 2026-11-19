package bootstrap

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/auth"
	deviceusecase "github.com/nice-platform/example/sharedservice-oauth/backend/internal/app/device/usecase"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/platform/config"
	"github.com/nice-platform/nice/pkg/niceauth/verify"
)

const baseScopes = "openid profile email offline_access"

func TestLoginKeepsTheTokensOnTheServer(t *testing.T) {
	e := newBFFEnv(t, nil)
	b := e.browser()
	state := b.startLogin()
	if c := b.cookie0; c == nil || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.Secure {
		t.Fatalf("cookie of the sign-in = %+v, want HttpOnly, SameSite=Lax, Path=/ and not Secure over http", c)
	}
	pending := b.cookie
	r := b.do(http.MethodGet, "/auth/callback?"+url.Values{"code": {"good"}, "state": {state}}.Encode(), nil)
	if r.Status != http.StatusFound || r.Header.Get("Location") != bffPublicURL+"/" {
		t.Fatalf("callback = %d to %q", r.Status, r.Header.Get("Location"))
	}
	if b.cookie == "" || b.cookie == pending {
		t.Fatal("the signed-in session must get a new ID, not the one of the pending sign-in")
	}
	r = b.do(http.MethodGet, "/auth/session", nil)
	if r.Status != http.StatusOK {
		t.Fatalf("session = %d %s", r.Status, r.Body)
	}
	if strings.Contains(r.Body, "eyJ") || strings.Contains(r.Body, "rt_") || strings.Contains(r.Body, testClientSecret) {
		t.Fatalf("the session body carries a token or the secret: %s", r.Body)
	}
	m := r.JSON(t)
	user := m["user"].(map[string]any)
	if user["id"] != bffUser || user["name"] != "Budi" || user["email"] != "budi@example.com" || m["csrf_token"] == "" {
		t.Fatalf("session = %v", m)
	}
	// The first project of the organization of the sign-in is chosen, so the devices show at once.
	if org, prj := contextOf(m); org != orgA || prj != projectP {
		t.Fatalf("context = %s/%s, want %s/%s", org, prj, orgA, projectP)
	}
	if got := permsOf(m); !slices.Equal(got, allPerms) {
		t.Fatalf("perms = %v, want %v", got, allPerms)
	}
	if got := e.nice.refreshScopes(); len(got) != 2 || got[0] != baseScopes ||
		got[1] != baseScopes+" nice:org:"+orgA+" nice:aud:nadi nice:prj:"+projectP {
		t.Fatalf("refreshes = %q, want the sign-in token then the token of the first project", got)
	}
}

func TestCallbackThatDoesNotBelongToTheSessionIsRefused(t *testing.T) {
	e := newBFFEnv(t, nil)
	b := e.browser()
	b.startLogin()
	r := b.do(http.MethodGet, "/auth/callback?code=good&state=forged", nil)
	if r.Status != http.StatusFound || r.Header.Get("Location") != bffPublicURL+"/?login_error=state_mismatch" {
		t.Fatalf("callback = %d to %q", r.Status, r.Header.Get("Location"))
	}
	if b.cookie != "" || e.auth.Store.Len() != 0 {
		t.Fatalf("cookie %q, %d sessions; want none", b.cookie, e.auth.Store.Len())
	}

	// A state started by another browser is refused for this one.
	other := e.browser()
	state := other.startLogin()
	b.startLogin()
	r = b.do(http.MethodGet, "/auth/callback?"+url.Values{"code": {"good"}, "state": {state}}.Encode(), nil)
	if !strings.HasSuffix(r.Header.Get("Location"), "login_error=state_mismatch") {
		t.Fatalf("callback with the state of another browser goes to %q", r.Header.Get("Location"))
	}
	if r = b.do(http.MethodGet, "/auth/session", nil); r.Status != http.StatusUnauthorized {
		t.Fatalf("session after a refused callback = %d", r.Status)
	}
	if len(e.nice.liveRefreshTokens()) != 0 {
		t.Fatalf("a refused callback got tokens: %v", e.nice.liveRefreshTokens())
	}
}

func TestAccessDeniedAtSignIn(t *testing.T) {
	e := newBFFEnv(t, nil)
	b := e.browser()
	state := b.startLogin()
	r := b.do(http.MethodGet, "/auth/callback?"+url.Values{"error": {"access_denied"}, "state": {state},
		"error_description": {"the service is not available in any of the user's organizations"}}.Encode(), nil)
	if r.Header.Get("Location") != bffPublicURL+"/?login_error=access_denied" {
		t.Fatalf("callback goes to %q", r.Header.Get("Location"))
	}
	if e.auth.Store.Len() != 0 {
		t.Fatalf("%d sessions after access_denied", e.auth.Store.Len())
	}
	r = b.do(http.MethodGet, "/auth/login?prompt=select_account", nil)
	if u, _ := url.Parse(r.Header.Get("Location")); u == nil || u.Query().Get("prompt") != "select_account" {
		t.Fatalf("login with another account goes to %q", r.Header.Get("Location"))
	}
}

func TestSessionWithoutCookieIs401(t *testing.T) {
	e := newBFFEnv(t, nil)
	wantError(t, e.browser().do(http.MethodGet, "/auth/session", nil), http.StatusUnauthorized, "unauthenticated", "")
	b := e.browser()
	b.cookie = "made-up"
	wantError(t, b.do(http.MethodGet, "/auth/session", nil), http.StatusUnauthorized, "unauthenticated", "")
	if b.cookie != "" {
		t.Fatal("an unknown session cookie was not cleared")
	}
	b.cookie = "made-up"
	wantError(t, b.do(http.MethodGet, "/devices", nil), http.StatusUnauthorized, "unauthenticated", "")
}

func TestSessionEndedByNICE(t *testing.T) {
	e := newBFFEnv(t, nil)
	b := e.browser()
	b.login()
	e.nice.set(func(f *bffNICE) { f.invalidGrant = true })
	r := b.do(http.MethodPost, "/auth/context", map[string]string{"org": orgA, "project": projectQ})
	wantError(t, r, http.StatusUnauthorized, "session_ended", "")
	if b.cookie != "" {
		t.Fatal("the cookie of an ended session was not cleared")
	}
	if e.auth.Store.Len() != 0 {
		t.Fatalf("%d sessions after invalid_grant, want 0", e.auth.Store.Len())
	}
}

func TestLogoutRevokesTheRefreshToken(t *testing.T) {
	e := newBFFEnv(t, nil)
	b := e.browser()
	b.login()
	live := e.nice.liveRefreshTokens()
	cookie := b.cookie
	if r := b.do(http.MethodPost, "/auth/logout", nil); r.Status != http.StatusNoContent {
		t.Fatalf("logout = %d %s", r.Status, r.Body)
	}
	if b.cookie != "" || e.auth.Store.Len() != 0 {
		t.Fatalf("cookie %q, %d sessions after logout", b.cookie, e.auth.Store.Len())
	}
	if got := e.nice.liveRefreshTokens(); len(got) != len(live)-1 {
		t.Fatalf("live refresh tokens %v -> %v, want the session's revoked", live, got)
	}
	b.cookie = cookie // a browser that kept the old cookie
	if r := b.do(http.MethodGet, "/auth/session", nil); r.Status != http.StatusUnauthorized {
		t.Fatalf("session after logout = %d", r.Status)
	}
}

func TestLogoutWhileNICEIsDownKeepsTheSession(t *testing.T) {
	e := newBFFEnv(t, nil)
	b := e.browser()
	b.login()
	e.nice.set(func(f *bffNICE) { f.revokeDown = true })
	wantError(t, b.do(http.MethodPost, "/auth/logout", nil), http.StatusServiceUnavailable, "temporarily_unavailable", "")
	if b.cookie == "" {
		t.Fatal("the cookie was cleared although the refresh token still works")
	}
	b.session()
	e.nice.set(func(f *bffNICE) { f.revokeDown = false })
	if r := b.do(http.MethodPost, "/auth/logout", nil); r.Status != http.StatusNoContent {
		t.Fatalf("logout once NICE is back = %d %s", r.Status, r.Body)
	}
}

func TestContextsComeFromNICEOverAllPages(t *testing.T) {
	e := newBFFEnv(t, nil)
	b := e.browser()
	b.login()
	r := b.do(http.MethodGet, "/auth/contexts", nil)
	if r.Status != http.StatusOK {
		t.Fatalf("contexts = %d %s", r.Status, r.Body)
	}
	m := r.JSON(t)
	var got []string
	for _, o := range m["organizations"].([]any) {
		org := o.(map[string]any)
		projects := org["projects"].([]any)
		if len(projects) == 0 {
			got = append(got, org["id"].(string)+"/-")
		}
		for _, p := range projects {
			got = append(got, org["id"].(string)+"/"+p.(map[string]any)["id"].(string))
		}
	}
	want := []string{orgA + "/" + projectP, orgA + "/" + projectQ, orgB + "/" + projectR, orgC + "/-"}
	if !slices.Equal(got, want) || m["current_organization_id"] != orgA {
		t.Fatalf("contexts = %v (current %v), want %v", got, m["current_organization_id"], want)
	}
	if !strings.Contains(r.Body, `"name":"Gudang"`) {
		t.Fatalf("contexts carry no names: %s", r.Body)
	}
}

func TestContextsWhenNICERefuses(t *testing.T) {
	e := newBFFEnv(t, nil)
	e.nice.set(func(f *bffNICE) { f.exchangeDown = true })
	b := e.browser()
	if m := b.login(); m["context"] != nil {
		t.Fatalf("session = %v, want a sign-in without a context when the discovery fails", m)
	}
	wantError(t, b.do(http.MethodGet, "/auth/contexts", nil), http.StatusBadGateway, "contexts_unavailable", "invalid_grant")

	e = newBFFEnv(t, nil)
	b = e.browser()
	b.login()
	e.nice.set(func(f *bffNICE) { f.contextsDown = true })
	wantError(t, b.do(http.MethodGet, "/auth/contexts", nil), http.StatusBadGateway, "contexts_unavailable", "permission_denied")
	// The active context and the devices still work.
	if r := b.do(http.MethodGet, "/devices", nil); r.Status != http.StatusOK {
		t.Fatalf("devices after a failed discovery = %d %s", r.Status, r.Body)
	}
}

func TestSwitchOrganizationWithOneRefresh(t *testing.T) {
	e := newBFFEnv(t, nil)
	b := e.browser()
	b.login()
	before := len(e.nice.refreshScopes())
	r := b.do(http.MethodPost, "/auth/context", map[string]string{"org": orgB, "project": projectR})
	if r.Status != http.StatusOK {
		t.Fatalf("switch = %d %s", r.Status, r.Body)
	}
	if org, prj := contextOf(r.JSON(t)); org != orgB || prj != projectR {
		t.Fatalf("context after the switch = %s/%s", org, prj)
	}
	got := e.nice.refreshScopes()[before:]
	if want := baseScopes + " nice:org:" + orgB + " nice:aud:nadi nice:prj:" + projectR; len(got) != 1 || got[0] != want {
		t.Fatalf("refreshes of the switch = %q, want one %q", got, want)
	}
	if org, prj := contextOf(b.session()); org != orgB || prj != projectR {
		t.Fatalf("session after the switch = %s/%s", org, prj)
	}
}

func TestSwitchRefusedKeepsTheContextAndTheRefreshToken(t *testing.T) {
	e := newBFFEnv(t, nil)
	b := e.browser()
	b.login()
	live := e.nice.liveRefreshTokens()
	e.nice.set(func(f *bffNICE) { f.refusedOrgs[orgB] = true })
	r := b.do(http.MethodPost, "/auth/context", map[string]string{"org": orgB, "project": projectR})
	wantError(t, r, http.StatusConflict, "context_refused", "access_denied")
	if org, prj := contextOf(b.session()); org != orgA || prj != projectP {
		t.Fatalf("context after a refused switch = %s/%s, want %s/%s", org, prj, orgA, projectP)
	}
	if got := e.nice.liveRefreshTokens(); !slices.Equal(got, live) {
		t.Fatalf("refresh tokens %v -> %v, want unchanged", live, got)
	}
	wantError(t, b.do(http.MethodPost, "/auth/context", map[string]string{"org": orgA, "project": "prj_gone"}),
		http.StatusConflict, "context_refused", "invalid_scope")
	wantError(t, b.do(http.MethodPost, "/auth/context", map[string]string{"org": orgA}), http.StatusBadRequest, "invalid_argument", "")
}

func TestTwoSwitchesAtOnceFromOneSession(t *testing.T) {
	e := newBFFEnv(t, nil)
	b := e.browser()
	b.login()
	var wg sync.WaitGroup
	status := make([]int, 2)
	targets := []map[string]string{{"org": orgA, "project": projectQ}, {"org": orgB, "project": projectR}}
	headers := []string{"Cookie", auth.CookieName + "=" + b.cookie, "Origin", bffPublicURL, "X-CSRF-Token", b.csrf}
	for i, target := range targets {
		wg.Go(func() { status[i] = e.do(http.MethodPost, "/auth/context", "", target, headers...).Status })
	}
	wg.Wait()
	if status[0] != http.StatusOK || status[1] != http.StatusOK {
		t.Fatalf("two switches at once = %v, want both 200 (the refresh token is used one at a time)", status)
	}
	b.session()
	if r := b.do(http.MethodGet, "/devices", nil); r.Status != http.StatusOK {
		t.Fatalf("devices after the switches = %d %s", r.Status, r.Body)
	}
}

func TestCookieRequestGetsTheTokenOfTheActiveContext(t *testing.T) {
	e := newBFFEnv(t, &fakeChecker{allowed: true})
	b := e.browser()
	b.login()
	r := b.do(http.MethodPost, "/devices", map[string]any{"name": "Sensor A", "serial_number": "SN-1", "status": "active"})
	if r.Status != http.StatusCreated || r.JSON(t)["created_by"] != bffUser {
		t.Fatalf("create in A/P = %d %s", r.Status, r.Body)
	}
	idA := r.JSON(t)["id"].(string)

	// Q gives read only: the permission comes from the token of the context.
	b.do(http.MethodPost, "/auth/context", map[string]string{"org": orgA, "project": projectQ})
	wantError(t, b.do(http.MethodPost, "/devices", map[string]any{"name": "x", "serial_number": "SN-1", "status": "active"}), http.StatusForbidden, "insufficient_scope", "permission_denied")

	// The tenant follows the context.
	b.do(http.MethodPost, "/auth/context", map[string]string{"org": orgB, "project": projectR})
	r = b.do(http.MethodGet, "/devices", nil)
	if r.Status != http.StatusOK || strings.Contains(r.Body, idA) {
		t.Fatalf("devices of B/R = %d %s, must not hold the device of A", r.Status, r.Body)
	}
	if r = b.do(http.MethodPost, "/devices", map[string]any{"name": "Sensor B", "serial_number": "SN-1", "status": "active"}); r.Status != http.StatusCreated {
		t.Fatalf("create in B/R = %d %s", r.Status, r.Body)
	}
	for sc, want := range map[domain.Scope]int{
		{OrgID: orgA, ProjectID: projectP}: 1, {OrgID: orgA, ProjectID: projectQ}: 0, {OrgID: orgB, ProjectID: projectR}: 1,
	} {
		if devices, _ := e.store.List(context.Background(), sc); len(devices) != want {
			t.Errorf("devices of %s/%s = %d, want %d", sc.OrgID, sc.ProjectID, len(devices), want)
		}
	}
	if r = b.do(http.MethodDelete, "/devices/"+r.JSON(t)["id"].(string), nil); r.Status != http.StatusNoContent {
		t.Fatalf("delete with the cookie = %d %s", r.Status, r.Body)
	}
}

func TestBearerCallerIsNotChangedByTheCookie(t *testing.T) {
	e := newBFFEnv(t, nil)
	b := e.browser()
	b.login()
	b.do(http.MethodPost, "/devices", map[string]any{"name": "Sensor A", "serial_number": "SN-1", "status": "active"})

	// agent-backend and Service Accounts send Authorization: the cookie is ignored, and so is the CSRF rule.
	cl := claims(orgB, allPerms...)
	cl["iss"], cl["prj"] = e.nice.URL, projectR
	tok := sign(t, cl)
	r := b.e.do(http.MethodPost, "/devices", tok, map[string]any{"name": "Dari agen", "serial_number": "SN-1", "status": "active"},
		"Cookie", auth.CookieName+"="+b.cookie, "Origin", "http://evil.test")
	if r.Status != http.StatusCreated {
		t.Fatalf("Bearer with a cookie = %d %s", r.Status, r.Body)
	}
	if devices, _ := e.store.List(context.Background(), domain.Scope{OrgID: orgB, ProjectID: projectR}); len(devices) != 1 {
		t.Fatalf("devices of the Bearer token's scope = %v", devices)
	}
	if r = e.do(http.MethodPost, "/devices", tok, map[string]any{"name": "Tanpa cookie", "serial_number": "SN-1", "status": "active"}); r.Status != http.StatusCreated {
		t.Fatalf("Bearer without a cookie = %d %s", r.Status, r.Body)
	}
}

func TestCSRF(t *testing.T) {
	e := newBFFEnv(t, nil)
	b := e.browser()
	b.login()
	scope := domain.Scope{OrgID: orgA, ProjectID: projectP}
	body := map[string]any{"name": "Sensor", "serial_number": "SN-1", "status": "active"}
	csrf := b.csrf

	b.csrf = ""
	wantError(t, b.do(http.MethodPost, "/devices", body), http.StatusForbidden, "csrf", "")
	wantError(t, b.do(http.MethodPost, "/devices", body, "X-CSRF-Token", "wrong"), http.StatusForbidden, "csrf", "")
	// A form posted from another site: the cookie goes along, the header and the Origin do not match.
	wantError(t, b.do(http.MethodPost, "/devices", body, "Origin", "http://evil.test", "X-CSRF-Token", csrf), http.StatusForbidden, "csrf", "")
	wantError(t, b.do(http.MethodPost, "/auth/context", map[string]string{"org": orgB, "project": projectR}, "Origin", "http://evil.test"),
		http.StatusForbidden, "csrf", "")
	wantError(t, b.do(http.MethodPost, "/auth/logout", nil, "X-CSRF-Token", "wrong"), http.StatusForbidden, "csrf", "")
	if devices, _ := e.store.List(context.Background(), scope); len(devices) != 0 {
		t.Fatalf("a refused write created %v", devices)
	}
	if org, _ := contextOf(b.session()); org != orgA {
		t.Fatalf("a refused switch moved the session to %s", org)
	}
}

func TestSessionWithoutAContext(t *testing.T) {
	e := newBFFEnv(t, nil)
	e.nice.set(func(f *bffNICE) { f.firstOrg = orgC })
	b := e.browser()
	m := b.login()
	if m["context"] != nil || len(m["perms"].([]any)) != 0 {
		t.Fatalf("session of an organization without projects = %v, want no context", m)
	}
	wantError(t, b.do(http.MethodGet, "/devices", nil), http.StatusConflict, "context_required", "")
}

func TestUIServedFromTheBuild(t *testing.T) {
	dist := t.TempDir()
	for name, content := range map[string]string{"index.html": "<!doctype html><title>NADI</title>", "assets/app.js": "console.log(1)"} {
		p := filepath.Join(dist, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	e := newTestEnv(t, nil)
	vc := VerifierConfig(config.Config{Issuer: testIssuer}, e.store, slog.New(slog.DiscardHandler))
	vc.StaticJWKS = testJWKS(t) // nothing is fetched: the issuer of the tests is a name, not an address
	v, err := verify.New(vc)
	if err != nil {
		t.Fatal(err)
	}
	h := func(dir string) *testEnv {
		env := *e
		env.handler = rest.NewHandler(v, &deviceusecase.Service{Repo: e.store}, rest.Options{WebDir: dir})
		return &env
	}
	withUI := h(dist)
	for path, want := range map[string]string{"/": "<title>NADI</title>", "/perangkat/123": "<title>NADI</title>", "/assets/app.js": "console.log(1)"} {
		if r := withUI.do(http.MethodGet, path, "", nil); r.Status != http.StatusOK || !strings.Contains(r.Body, want) {
			t.Errorf("GET %s = %d %q", path, r.Status, r.Body)
		}
	}
	for _, path := range []string{"/assets/missing.js", "/auth/unknown"} {
		if r := withUI.do(http.MethodGet, path, "", nil); r.Status != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, r.Status)
		}
	}
	if r := h(filepath.Join(dist, "missing")).do(http.MethodGet, "/", "", nil); r.Status != http.StatusNotFound {
		t.Fatalf("GET / without a build = %d, want 404", r.Status)
	}
}
