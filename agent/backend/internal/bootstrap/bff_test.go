package bootstrap

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ABA-Developer/nice-auth-sdk/verify"

	mcpadapter "github.com/nice-platform/example/agent/backend/internal/adapter/mcp"
	"github.com/nice-platform/example/agent/backend/internal/adapter/rest/auth"
	calldto "github.com/nice-platform/example/agent/backend/internal/app/call/dto"
	"github.com/nice-platform/example/agent/backend/internal/domain"
)

// chatProbe stands in for the chat API: it records the caller the session middleware put on the request.
type chatProbe struct {
	mu      sync.Mutex
	callers []calldto.Caller
	hits    int
}

func (p *chatProbe) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c, ok := auth.CallerFrom(r.Context())
	p.mu.Lock()
	p.hits++
	if ok {
		p.callers = append(p.callers, c)
	}
	p.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (p *chatProbe) seen() (int, []calldto.Caller) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.hits, slices.Clone(p.callers)
}

func TestLoginKeepsTheTokensOnTheServer(t *testing.T) {
	e := newBFFEnv(t, nil)
	b := e.browser()
	state := b.startLogin()
	if c := b.cookie0; c == nil || c.Name != "agen_session" || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.Secure {
		t.Fatalf("cookie of the sign-in = %+v, want agen_session, HttpOnly, SameSite=Lax, Path=/ and not Secure over http", c)
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
	if strings.Contains(r.Body, "eyJ") || strings.Contains(r.Body, "rt_") || strings.Contains(r.Body, testSecret) {
		t.Fatalf("the session body carries a token or the secret: %s", r.Body)
	}
	m := r.JSON(t)
	user := m["user"].(map[string]any)
	if user["id"] != bffUser || user["name"] != "Budi" || user["email"] != "budi@example.com" || m["csrf_token"] == "" {
		t.Fatalf("session = %v", m)
	}
	if org, prj := contextOf(m); org != orgA || prj != prjP {
		t.Fatalf("context = %s/%s, want %s/%s", org, prj, orgA, prjP)
	}
	if got := permsOf(m); !slices.Equal(got, []string{domain.PermUse}) {
		t.Fatalf("perms = %v, want %s", got, domain.PermUse)
	}
	if got := e.nice.refreshScopes(); len(got) != 2 || got[0] != bffScopes ||
		got[1] != bffScopes+" nice:org:"+orgA+" nice:aud:agen nice:prj:"+prjP {
		t.Fatalf("refreshes = %q, want the sign-in token then the agen token of the first project", got)
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

func TestSessionWithoutCookieIs401(t *testing.T) {
	e := newBFFEnv(t, nil)
	wantBFFError(t, e.browser().do(http.MethodGet, "/auth/session", nil), http.StatusUnauthorized, "unauthenticated", "")
	b := e.browser()
	b.cookie = "made-up"
	wantBFFError(t, b.do(http.MethodGet, "/auth/session", nil), http.StatusUnauthorized, "unauthenticated", "")
	if b.cookie != "" {
		t.Fatal("an unknown session cookie was not cleared")
	}
	b.cookie = "made-up"
	wantBFFError(t, b.do(http.MethodGet, "/chat/conversations", nil), http.StatusUnauthorized, "unauthenticated", "")
}

func TestSessionEndedByNICE(t *testing.T) {
	e := newBFFEnv(t, nil)
	b := e.browser()
	b.login()
	e.nice.set(func(f *bffNICE) { f.invalidGrant = true })
	r := b.do(http.MethodPost, "/auth/context", map[string]string{"org": orgB, "project": prjR})
	wantBFFError(t, r, http.StatusUnauthorized, "session_ended", "")
	if b.cookie != "" {
		t.Fatal("the cookie of an ended session was not cleared")
	}
	if e.auth.Store.Len() != 0 {
		t.Fatalf("%d sessions after invalid_grant, want 0", e.auth.Store.Len())
	}
}

func TestLogoutRevokesTheRefreshToken(t *testing.T) {
	probe := &chatProbe{}
	e := newBFFEnv(t, probe)
	b := e.browser()
	b.login()
	live := e.nice.liveRefreshTokens()
	cookie := b.cookie
	r := b.do(http.MethodPost, "/auth/logout", nil)
	if r.Status != http.StatusOK {
		t.Fatalf("logout = %d %s", r.Status, r.Body)
	}
	end, err := url.Parse(r.JSON(t)["end_session_url"].(string))
	if err != nil || end.String() != e.nice.URL+"/oauth/end_session?"+end.RawQuery {
		t.Fatalf("end_session_url = %v, %v", end, err)
	}
	if q := end.Query(); q.Get("client_id") != testClientID || q.Get("post_logout_redirect_uri") != bffPublicURL+"/" || q.Get("id_token_hint") != bffIDToken {
		t.Fatalf("end-session query = %v", q)
	}
	if b.cookie != "" || e.auth.Store.Len() != 0 {
		t.Fatalf("cookie %q, %d sessions after logout", b.cookie, e.auth.Store.Len())
	}
	if got := e.nice.liveRefreshTokens(); len(got) != len(live)-1 {
		t.Fatalf("live refresh tokens %v -> %v, want the session's revoked", live, got)
	}
	b.cookie = cookie // a browser that kept the old cookie
	wantBFFError(t, b.do(http.MethodGet, "/chat/conversations", nil), http.StatusUnauthorized, "unauthenticated", "")
	if hits, _ := probe.seen(); hits != 0 {
		t.Fatalf("the chat got %d requests after logout", hits)
	}
}

func TestLogoutWhileNICEIsDownKeepsTheSession(t *testing.T) {
	e := newBFFEnv(t, nil)
	b := e.browser()
	b.login()
	e.nice.set(func(f *bffNICE) { f.revokeDown = true })
	wantBFFError(t, b.do(http.MethodPost, "/auth/logout", nil), http.StatusServiceUnavailable, "temporarily_unavailable", "")
	if b.cookie == "" {
		t.Fatal("the cookie was cleared although the refresh token still works")
	}
	b.session()
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
	want := []string{orgA + "/" + prjP, orgA + "/" + prjQ, orgB + "/" + prjR, orgC + "/-"}
	if !slices.Equal(got, want) || m["current_organization_id"] != orgA {
		t.Fatalf("contexts = %v (current %v), want %v", got, m["current_organization_id"], want)
	}
}

func TestSwitchContextWithOneRefresh(t *testing.T) {
	probe := &chatProbe{}
	e := newBFFEnv(t, probe)
	b := e.browser()
	b.login()
	before := len(e.nice.refreshScopes())
	r := b.do(http.MethodPost, "/auth/context", map[string]string{"org": orgB, "project": prjR})
	if r.Status != http.StatusOK {
		t.Fatalf("switch = %d %s", r.Status, r.Body)
	}
	if org, prj := contextOf(r.JSON(t)); org != orgB || prj != prjR {
		t.Fatalf("context after the switch = %s/%s", org, prj)
	}
	got := e.nice.refreshScopes()[before:]
	if want := bffScopes + " nice:org:" + orgB + " nice:aud:agen nice:prj:" + prjR; len(got) != 1 || got[0] != want {
		t.Fatalf("refreshes of the switch = %q, want one %q", got, want)
	}
	// Later chat calls run in the new context.
	if r := b.do(http.MethodGet, "/chat/conversations", nil); r.Status != http.StatusNoContent {
		t.Fatalf("chat after the switch = %d %s", r.Status, r.Body)
	}
	if _, callers := probe.seen(); len(callers) != 1 || callers[0].OrgID != orgB || callers[0].ProjectID != prjR {
		t.Fatalf("callers = %+v, want one in %s/%s", callers, orgB, prjR)
	}
}

func TestSwitchRefusedKeepsTheContextAndTheRefreshToken(t *testing.T) {
	e := newBFFEnv(t, nil)
	b := e.browser()
	b.login()
	live := e.nice.liveRefreshTokens()
	e.nice.set(func(f *bffNICE) { f.refusedOrgs[orgB] = true })
	wantBFFError(t, b.do(http.MethodPost, "/auth/context", map[string]string{"org": orgB, "project": prjR}),
		http.StatusConflict, "context_refused", "access_denied")
	if org, prj := contextOf(b.session()); org != orgA || prj != prjP {
		t.Fatalf("context after a refused switch = %s/%s", org, prj)
	}
	if got := e.nice.liveRefreshTokens(); !slices.Equal(got, live) {
		t.Fatalf("refresh tokens %v -> %v, want unchanged", live, got)
	}
	wantBFFError(t, b.do(http.MethodPost, "/auth/context", map[string]string{"org": orgA}), http.StatusBadRequest, "invalid_argument", "")
}

func TestCSRF(t *testing.T) {
	probe := &chatProbe{}
	e := newBFFEnv(t, probe)
	b := e.browser()
	b.login()
	csrf := b.csrf
	b.csrf = ""
	body := map[string]string{"title": "x"}
	wantBFFError(t, b.do(http.MethodPost, "/chat/conversations", body), http.StatusForbidden, "csrf", "")
	wantBFFError(t, b.do(http.MethodPost, "/chat/conversations", body, "X-CSRF-Token", "wrong"), http.StatusForbidden, "csrf", "")
	// A form posted from another site: the cookie goes along, the Origin does not match.
	wantBFFError(t, b.do(http.MethodPost, "/chat/conversations", body, "Origin", "http://evil.test", "X-CSRF-Token", csrf),
		http.StatusForbidden, "csrf", "")
	wantBFFError(t, b.do(http.MethodPost, "/auth/context", map[string]string{"org": orgB, "project": prjR}, "Origin", "http://evil.test"),
		http.StatusForbidden, "csrf", "")
	wantBFFError(t, b.do(http.MethodPost, "/auth/logout", nil, "X-CSRF-Token", "wrong"), http.StatusForbidden, "csrf", "")
	if hits, _ := probe.seen(); hits != 0 {
		t.Fatalf("the chat got %d refused writes", hits)
	}
	if org, _ := contextOf(b.session()); org != orgA {
		t.Fatalf("a refused switch moved the session to %s", org)
	}
	if r := b.do(http.MethodPost, "/chat/conversations", body); r.Status != http.StatusNoContent {
		t.Fatalf("a write with the CSRF header and Origin = %d %s", r.Status, r.Body)
	}
}

func TestSignedInChatRequestCarriesTheCaller(t *testing.T) {
	probe := &chatProbe{}
	e := newBFFEnv(t, probe)
	b := e.browser()
	b.login()
	if r := b.do(http.MethodGet, "/chat/x", nil); r.Status != http.StatusNoContent {
		t.Fatalf("chat = %d %s", r.Status, r.Body)
	}
	_, callers := probe.seen()
	if len(callers) != 1 {
		t.Fatalf("callers = %+v, want one", callers)
	}
	c := callers[0]
	if c.Subject != bffUser || c.OrgID != orgA || c.ProjectID != prjP || !strings.HasPrefix(c.Token, "eyJ") {
		t.Fatalf("caller = %+v", c)
	}
	if aud := readClaims(t, c.Token)["aud"]; aud != domain.ServiceSlug {
		t.Fatalf("caller token aud = %v, want %s", aud, domain.ServiceSlug)
	}
}

func TestChatPlaceholderAnswers404(t *testing.T) {
	e := newBFFEnv(t, nil)
	b := e.browser()
	b.login()
	if r := b.do(http.MethodGet, "/chat/x", nil); r.Status != http.StatusNotFound {
		t.Fatalf("chat placeholder = %d %s, want 404", r.Status, r.Body)
	}
}

func TestChatNeedsThePermissionOfTheAgent(t *testing.T) {
	probe := &chatProbe{}
	e := newBFFEnv(t, probe)
	b := e.browser()
	b.login()
	if r := b.do(http.MethodPost, "/auth/context", map[string]string{"org": orgA, "project": prjQ}); r.Status != http.StatusOK {
		t.Fatalf("switch = %d %s", r.Status, r.Body)
	}
	wantBFFError(t, b.do(http.MethodGet, "/chat/conversations", nil), http.StatusForbidden, "insufficient_scope", "permission_denied")
	if hits, _ := probe.seen(); hits != 0 {
		t.Fatalf("the chat got %d requests without %s", hits, domain.PermUse)
	}
}

func TestChatWithoutAContext(t *testing.T) {
	e := newBFFEnv(t, &chatProbe{})
	e.nice.set(func(f *bffNICE) { f.firstOrg = orgC })
	b := e.browser()
	if m := b.login(); m["context"] != nil {
		t.Fatalf("session of an organization without projects = %v, want no context", m)
	}
	wantBFFError(t, b.do(http.MethodGet, "/chat/conversations", nil), http.StatusConflict, "context_required", "")
}

func TestChatRefusesABearerToken(t *testing.T) {
	probe := &chatProbe{}
	e := newBFFEnv(t, probe)
	tok := sign(t, userClaims(e.nice.URL, domain.ServiceSlug, orgA, prjP, domain.PermUse))
	wantBFFError(t, e.do(http.MethodGet, "/chat/conversations", tok, nil), http.StatusUnauthorized, "unauthenticated", "session_required")

	// Not even next to a valid session: the chat is a cookie API only.
	b := e.browser()
	b.login()
	wantBFFError(t, e.do(http.MethodGet, "/chat/conversations", tok, nil, "Cookie", auth.CookieName+"="+b.cookie),
		http.StatusUnauthorized, "unauthenticated", "session_required")
	if hits, _ := probe.seen(); hits != 0 {
		t.Fatalf("the chat got %d Bearer requests", hits)
	}
}

func TestMCPIgnoresTheSessionCookie(t *testing.T) {
	e := newBFFEnv(t, nil)
	b := e.browser()
	b.login()
	r := b.do(http.MethodPost, mcpadapter.MCPPath, initialize, "Accept", "application/json, text/event-stream")
	if r.Status != http.StatusUnauthorized {
		t.Fatalf("/mcp with a session cookie = %d %s, want 401", r.Status, r.Body)
	}
	want := `resource_metadata="` + bffPublicURL + `/.well-known/oauth-protected-resource"`
	if h := r.Header.Get("WWW-Authenticate"); !strings.HasPrefix(h, "Bearer ") || !strings.Contains(h, want) {
		t.Fatalf("WWW-Authenticate = %q, want Bearer with %s", h, want)
	}
	wantBFFError(t, r, http.StatusUnauthorized, "invalid_token", "missing_token")
}

func TestPublicEndpointsStayWithTheDashboard(t *testing.T) {
	e := newBFFEnv(t, nil)
	for _, p := range []string{"/healthz", "/openapi.json", "/.well-known/oauth-protected-resource", "/.well-known/oauth-protected-resource/mcp"} {
		if r := e.do(http.MethodGet, p, "", nil); r.Status != http.StatusOK {
			t.Errorf("GET %s = %d", p, r.Status)
		}
	}
}

// staleBFFEnv signs in with tokens issued ten seconds ago and then starts the access change feed with a change of the
// user in organization A five seconds ago, so the agen token of the active context A/P is stale.
func staleBFFEnv(t *testing.T, chat http.Handler) (*bffEnv, *browser) {
	t.Helper()
	e := newBFFEnv(t, chat)
	e.nice.set(func(f *bffNICE) {
		f.iatShift = -10 * time.Second
		f.changes = []map[string]string{{"seq": "1", "kind": "binding_deleted", "key": verify.FeedKey("p", bffUser, orgA),
			"org_key": verify.FeedKey("o", orgA), "occurred_at": time.Now().Add(-5 * time.Second).UTC().Format(time.RFC3339)}}
	})
	b := e.browser()
	b.login()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := e.v.StartFeed(ctx, verify.FeedConfig{}); err != nil {
		t.Fatal(err)
	}
	return e, b
}

func TestStaleChatTokenIsRenewedOnce(t *testing.T) {
	probe := &chatProbe{}
	e, b := staleBFFEnv(t, probe)
	e.nice.set(func(f *bffNICE) { f.iatShift = 0 })
	before := len(e.nice.refreshScopes())
	if r := b.do(http.MethodGet, "/chat/conversations", nil); r.Status != http.StatusNoContent {
		t.Fatalf("chat with a stale cached token = %d %s, want a new token", r.Status, r.Body)
	}
	if n := len(e.nice.refreshScopes()) - before; n != 1 {
		t.Fatalf("%d refreshes, want one for the stale token", n)
	}
}

func TestStaleChatTokenTwiceIsNotRenewedAgain(t *testing.T) {
	e, b := staleBFFEnv(t, &chatProbe{})
	before := len(e.nice.refreshScopes())
	wantBFFError(t, b.do(http.MethodGet, "/chat/conversations", nil), http.StatusUnauthorized, "invalid_token", "token_stale")
	if n := len(e.nice.refreshScopes()) - before; n != 1 {
		t.Fatalf("%d refreshes, want exactly one", n)
	}
}

func TestUIServedFromTheBuild(t *testing.T) {
	dist := t.TempDir()
	for name, content := range map[string]string{"index.html": "<!doctype html><title>Agen</title>", "assets/app.js": "console.log(1)"} {
		p := filepath.Join(dist, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	e := newBFFEnv(t, nil)
	withUI := func(dir string) *bffEnv {
		env := *e
		cfg := e.cfg
		cfg.WebDir = dir
		env.handler = NewHandler(Deps{Verifier: e.v, Calls: e.calls, Config: cfg, Auth: e.auth.Handler})
		return &env
	}
	ui := withUI(dist)
	for path, want := range map[string]string{"/": "<title>Agen</title>", "/c/123": "<title>Agen</title>", "/assets/app.js": "console.log(1)"} {
		if r := ui.do(http.MethodGet, path, "", nil); r.Status != http.StatusOK || !strings.Contains(r.Body, want) {
			t.Errorf("GET %s = %d %q", path, r.Status, r.Body)
		}
	}
	for _, path := range []string{"/assets/missing.js", "/auth/unknown"} {
		if r := ui.do(http.MethodGet, path, "", nil); r.Status != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, r.Status)
		}
	}
	if r := ui.do(http.MethodPost, "/", "", nil); r.Status != http.StatusMethodNotAllowed {
		t.Errorf("POST / = %d, want 405", r.Status)
	}
	// The UI does not take over the routes of the agent.
	if r := ui.do(http.MethodGet, "/healthz", "", nil); r.Status != http.StatusOK || strings.Contains(r.Body, "<title>") {
		t.Errorf("GET /healthz with a UI = %d %q", r.Status, r.Body)
	}
	if r := ui.do(http.MethodPost, mcpadapter.MCPPath, "", initialize); r.Status != http.StatusUnauthorized {
		t.Errorf("POST /mcp with a UI = %d, want 401", r.Status)
	}
	if r := withUI(filepath.Join(dist, "missing")).do(http.MethodGet, "/", "", nil); r.Status != http.StatusNotFound {
		t.Fatalf("GET / without a build = %d, want 404", r.Status)
	}
}
