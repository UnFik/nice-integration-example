package bootstrap

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ABA-Developer/nice-auth-sdk/verify"

	"github.com/nice-platform/example/agent/backend/internal/adapter/rest/auth"
	callusecase "github.com/nice-platform/example/agent/backend/internal/app/call/usecase"
	"github.com/nice-platform/example/agent/backend/internal/domain"
	"github.com/nice-platform/example/agent/backend/internal/platform/config"
)

const (
	bffPublicURL = "http://localhost:5182"
	bffUser      = "usr_budi"
	bffIDToken   = "id.token.of-the-sign-in"
	prjR         = "prj_rrrrrrrr"
	bffScopes    = "openid profile email offline_access"
)

type bffProject struct {
	id, name string
	perms    []string
}

type bffOrg struct {
	id, name string
	projects []bffProject
	actAs    bool
}

// bffNICE is a NICE for the dashboard sign-in: the token endpoint (code, refresh with nice:org, exchange of the
// sign-in token for the NICE API), revoke, userinfo, /api/v1/me/contexts and the access change feed. It rotates
// refresh tokens the way NICE does: a refresh token used twice is invalid_grant.
type bffNICE struct {
	*httptest.Server
	t *testing.T

	mu           sync.Mutex
	orgs         []bffOrg
	firstOrg     string
	refresh      map[string]string // live refresh token -> organization it is in
	spent        map[string]bool
	seq          int
	refusedOrgs  map[string]bool
	invalidGrant bool
	exchangeDown bool
	revokeDown   bool
	refreshes    []string // scope of every refresh
	revoked      []string
	// iatShift moves the issue time of the tokens it signs, to make a token older than a change of the feed.
	iatShift time.Duration
	changes  []map[string]string
}

func newBFFNICE(t *testing.T) *bffNICE {
	t.Helper()
	f := &bffNICE{t: t, firstOrg: orgA, refresh: map[string]string{}, spent: map[string]bool{}, refusedOrgs: map[string]bool{},
		orgs: []bffOrg{
			{orgA, "Nusapala", []bffProject{{prjP, "Gudang", []string{domain.PermUse}}, {prjQ, "Toko", nil}}, false},
			{orgB, "Banua", []bffProject{{prjR, "Pusat", []string{domain.PermUse}}}, false},
			{orgC, "Cakra", nil, true},
		}}
	f.Server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.Close)
	return f
}

func (f *bffNICE) set(change func(f *bffNICE)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	change(f)
}

func (f *bffNICE) refreshScopes() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.refreshes...)
}

func (f *bffNICE) liveRefreshTokens() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for rt := range f.refresh {
		out = append(out, rt)
	}
	slices.Sort(out)
	return out
}

func (f *bffNICE) org(id string) (bffOrg, bool) {
	for _, o := range f.orgs {
		if o.id == id {
			return o, true
		}
	}
	return bffOrg{}, false
}

func (f *bffNICE) token(c map[string]any) string {
	now := time.Now().Add(f.iatShift)
	base := map[string]any{"iss": f.URL, "sub": bffUser, "iat": now.Unix(), "nbf": now.Unix(), "exp": now.Add(30 * time.Minute).Unix(),
		"nice_principal_type": "user"}
	for k, v := range c {
		base[k] = v
	}
	return sign(f.t, base)
}

func (f *bffNICE) serve(w http.ResponseWriter, r *http.Request) {
	if r.Method+" "+r.URL.Path == "GET /api/v1/access-changes" {
		// Outside the lock: a long poll holds until the reader gives up.
		f.serveFeed(w, r)
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	switch r.Method + " " + r.URL.Path {
	case "POST /oauth/token":
		f.serveToken(w, r)
	case "POST /oauth/revoke":
		if f.revokeDown {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_ = r.ParseForm()
		delete(f.refresh, r.PostForm.Get("token"))
		f.revoked = append(f.revoked, r.PostForm.Get("token"))
		w.WriteHeader(http.StatusOK)
	case "GET /oauth/userinfo":
		_ = json.NewEncoder(w).Encode(map[string]string{"sub": bffUser, "name": "Budi", "email": "budi@example.com"})
	case "GET /api/v1/me/contexts":
		f.serveContexts(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (f *bffNICE) serveToken(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	form := r.PostForm
	id, secret, ok := r.BasicAuth()
	if !ok {
		id, secret = form.Get("client_id"), form.Get("client_secret")
	}
	if id != testClientID || secret != testSecret {
		tokenError(w, http.StatusUnauthorized, "invalid_client", "")
		return
	}
	var org, aud, prj string
	for _, s := range strings.Fields(form.Get("scope")) {
		switch {
		case strings.HasPrefix(s, "nice:org:"):
			org = strings.TrimPrefix(s, "nice:org:")
		case strings.HasPrefix(s, "nice:aud:"):
			aud = strings.TrimPrefix(s, "nice:aud:")
		case strings.HasPrefix(s, "nice:prj:"):
			prj = strings.TrimPrefix(s, "nice:prj:")
		}
	}
	switch form.Get("grant_type") {
	case "authorization_code":
		if form.Get("code") != "good" || form.Get("code_verifier") == "" || form.Get("redirect_uri") != bffPublicURL+config.CallbackPath {
			tokenError(w, http.StatusBadRequest, "invalid_grant", "")
			return
		}
		f.issue(w, f.firstOrg, "", "", bffIDToken)
	case "refresh_token":
		f.refreshes = append(f.refreshes, form.Get("scope"))
		rt := form.Get("refresh_token")
		current, live := f.refresh[rt]
		if f.invalidGrant || !live || f.spent[rt] {
			tokenError(w, http.StatusBadRequest, "invalid_grant", "")
			return
		}
		if org == "" {
			org = current
		}
		o, known := f.org(org)
		if !known || f.refusedOrgs[org] {
			// The refresh token stays valid, as NICE does for a refused move.
			tokenError(w, http.StatusBadRequest, "access_denied", "organization is not usable")
			return
		}
		if aud != "" && !slices.ContainsFunc(o.projects, func(p bffProject) bool { return p.id == prj }) {
			tokenError(w, http.StatusBadRequest, "invalid_scope", "project")
			return
		}
		delete(f.refresh, rt)
		f.spent[rt] = true
		f.issue(w, org, aud, prj, "")
	case "urn:ietf:params:oauth:grant-type:token-exchange":
		if f.exchangeDown {
			tokenError(w, http.StatusBadRequest, "invalid_grant", "subject_invalid")
			return
		}
		if aud != "nice" {
			tokenError(w, http.StatusBadRequest, "invalid_grant", "a sign-in token can only be exchanged for the NICE API")
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": f.token(map[string]any{"aud": "nice", "nice_org_id": f.firstOrg,
			"perms": []string{"nice.me.contexts.read"}, "act": map[string]string{"sub": testClientID}}), "expires_in": 1800})
	default:
		tokenError(w, http.StatusBadRequest, "unsupported_grant_type", "")
	}
}

// serveFeed replays the changes on a read since a time and then holds every long poll, as NICE does when nothing
// changes.
func (f *bffNICE) serveFeed(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("since") != "" {
		f.mu.Lock()
		changes := append([]map[string]string(nil), f.changes...)
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"changes": changes, "seq": strconv.Itoa(len(changes))})
		return
	}
	<-r.Context().Done()
}

// issue answers a token in org: for aud and prj, or the sign-in token without aud.
func (f *bffNICE) issue(w http.ResponseWriter, org, aud, prj, idToken string) {
	f.seq++
	rt := "rt_" + strconv.Itoa(f.seq)
	f.refresh[rt] = org
	c := map[string]any{"aud": testClientID, "azp": testClientID, "nice_org_id": org}
	if aud != "" {
		o, _ := f.org(org)
		perms := []string{}
		for _, p := range o.projects {
			if p.id == prj && p.perms != nil {
				perms = p.perms
			}
		}
		c = map[string]any{"aud": aud, "azp": testClientID, "nice_org_id": org, "prj": prj, "perms": perms, "pv": 1}
	}
	body := map[string]any{"access_token": f.token(c), "refresh_token": rt, "expires_in": 1800, "token_type": "Bearer"}
	if idToken != "" {
		body["id_token"] = idToken
	}
	_ = json.NewEncoder(w).Encode(body)
}

// serveContexts answers one organization per page, so the backend must follow the pages.
func (f *bffNICE) serveContexts(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	i, _ := strconv.Atoi(r.URL.Query().Get("page_token"))
	o := f.orgs[i]
	projects := []map[string]any{}
	for _, p := range o.projects {
		projects = append(projects, map[string]any{"id": p.id, "name": p.name, "slug": strings.ToLower(p.name), "services": []string{domain.ServiceSlug}})
	}
	page := map[string]any{}
	if i+1 < len(f.orgs) {
		page["next_page_token"] = strconv.Itoa(i + 1)
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"current_organization_id": f.firstOrg, "services": []string{domain.ServiceSlug},
		"organizations": []map[string]any{{"id": o.id, "name": o.name, "slug": strings.ToLower(o.name), "projects": projects,
			"projects_truncated": false, "act_as": o.actAs}},
		"page": page})
}

// bffResponse is one answer of the agent, read from a recorder.
type bffResponse struct {
	Status int
	Header http.Header
	Body   string
}

func (r bffResponse) JSON(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(r.Body), &m); err != nil {
		t.Fatalf("body is not JSON: %s", r.Body)
	}
	return m
}

func wantBFFError(t *testing.T, r bffResponse, status int, code, description string) {
	t.Helper()
	if r.Status != status {
		t.Fatalf("status = %d, want %d; body %s", r.Status, status, r.Body)
	}
	m := r.JSON(t)
	if got, _ := m["error_description"].(string); m["error"] != code || got != description {
		t.Fatalf("error = %v / %v, want %s / %s", m["error"], m["error_description"], code, description)
	}
	if strings.Contains(r.Body, "eyJ") {
		t.Fatalf("the error body echoes a token: %s", r.Body)
	}
}

// bffEnv is the agent as Run wires it, with the dashboard sign-in against bffNICE and chat as the handler of /chat/.
type bffEnv struct {
	t       *testing.T
	nice    *bffNICE
	auth    *Auth
	v       *verify.Verifier
	cfg     config.Config
	calls   *callusecase.Service
	handler http.Handler
}

func newBFFEnv(t *testing.T, chat http.Handler) *bffEnv {
	t.Helper()
	f := newBFFNICE(t)
	cfg := config.Config{Issuer: f.URL, ClientID: testClientID, ClientSecret: testSecret, PublicURL: bffPublicURL,
		RedirectURI: bffPublicURL + config.CallbackPath, PostLogoutURL: bffPublicURL + "/", SessionIdle: 8 * time.Hour}
	log := slog.New(slog.DiscardHandler)
	vc := VerifierConfig(cfg, log)
	vc.StaticJWKS = testJWKS(t)
	v, err := verify.New(vc)
	if err != nil {
		t.Fatal(err)
	}
	calls, err := NewCallService(cfg, http.DefaultClient, log)
	if err != nil {
		t.Fatal(err)
	}
	a, err := NewAuth(cfg, v, log)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(Deps{Verifier: v, Calls: calls, Config: cfg, Auth: a.Handler, Chat: chat})
	return &bffEnv{t: t, nice: f, auth: a, v: v, cfg: cfg, calls: calls, handler: h}
}

func (e *bffEnv) do(method, path, token string, body any, headers ...string) bffResponse {
	e.t.Helper()
	var rd io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		rd = strings.NewReader(b)
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			e.t.Fatal(err)
		}
		rd = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, rd)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if rd != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	return bffResponse{Status: rec.Code, Header: rec.Header(), Body: rec.Body.String()}
}

// browser keeps the session cookie the way a browser does and sends the CSRF header and Origin of the UI on writes.
type browser struct {
	e       *bffEnv
	cookie  string
	csrf    string
	cookie0 *http.Cookie
}

func (e *bffEnv) browser() *browser { return &browser{e: e} }

func (b *browser) do(method, path string, body any, headers ...string) bffResponse {
	b.e.t.Helper()
	if b.cookie != "" {
		headers = append(headers, "Cookie", auth.CookieName+"="+b.cookie)
	}
	if isWriteMethod(method) && !slices.Contains(headers, "Origin") {
		headers = append(headers, "Origin", bffPublicURL)
	}
	if isWriteMethod(method) && b.csrf != "" && !slices.Contains(headers, "X-CSRF-Token") {
		headers = append(headers, "X-CSRF-Token", b.csrf)
	}
	r := b.e.do(method, path, "", body, headers...)
	for _, c := range (&http.Response{Header: r.Header}).Cookies() {
		if c.Name != auth.CookieName {
			continue
		}
		b.cookie0 = c
		if c.MaxAge < 0 {
			b.cookie = ""
		} else {
			b.cookie = c.Value
		}
	}
	return r
}

func isWriteMethod(m string) bool {
	return m == http.MethodPost || m == http.MethodPut || m == http.MethodPatch || m == http.MethodDelete
}

// startLogin follows /auth/login and returns the state NICE would get.
func (b *browser) startLogin() string {
	b.e.t.Helper()
	r := b.do(http.MethodGet, "/auth/login", nil)
	if r.Status != http.StatusFound {
		b.e.t.Fatalf("login = %d %s", r.Status, r.Body)
	}
	u, err := url.Parse(r.Header.Get("Location"))
	if err != nil || !strings.HasPrefix(u.String(), b.e.nice.URL+"/oauth/authorize?") {
		b.e.t.Fatalf("login goes to %q, want the authorize endpoint of NICE", r.Header.Get("Location"))
	}
	return u.Query().Get("state")
}

// login signs in and reads the session, keeping its CSRF token.
func (b *browser) login() map[string]any {
	b.e.t.Helper()
	state := b.startLogin()
	r := b.do(http.MethodGet, "/auth/callback?"+url.Values{"code": {"good"}, "state": {state}}.Encode(), nil)
	if r.Status != http.StatusFound || r.Header.Get("Location") != bffPublicURL+"/" {
		b.e.t.Fatalf("callback = %d to %q %s", r.Status, r.Header.Get("Location"), r.Body)
	}
	return b.session()
}

func (b *browser) session() map[string]any {
	b.e.t.Helper()
	r := b.do(http.MethodGet, "/auth/session", nil)
	if r.Status != http.StatusOK {
		b.e.t.Fatalf("session = %d %s", r.Status, r.Body)
	}
	m := r.JSON(b.e.t)
	b.csrf, _ = m["csrf_token"].(string)
	return m
}

func contextOf(m map[string]any) (org, project string) {
	c, _ := m["context"].(map[string]any)
	org, _ = c["org"].(string)
	project, _ = c["project"].(string)
	return org, project
}

func permsOf(m map[string]any) []string {
	var out []string
	for _, p := range m["perms"].([]any) {
		out = append(out, p.(string))
	}
	return out
}
