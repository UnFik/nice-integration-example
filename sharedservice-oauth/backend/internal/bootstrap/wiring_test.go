package bootstrap

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ABA-Developer/nice-auth-sdk/verify"
	devicememory "github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/memory/device"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/platform/config"
)

func TestCheckerAndResolverNeedNoConfiguration(t *testing.T) {
	c := VerifierConfig(config.Config{Issuer: testIssuer}, devicememory.NewRepository(), slog.New(slog.DiscardHandler))
	if _, ok := c.Checker.(*verify.HTTPChecker); !ok {
		t.Fatalf("checker = %T, want *verify.HTTPChecker", c.Checker)
	}
	if _, ok := c.Resolver.(*verify.HTTPResolver); !ok {
		t.Fatalf("resolver = %T, want *verify.HTTPResolver", c.Resolver)
	}
	if c.Service != domain.ServiceSlug || c.Tenants == nil {
		t.Fatalf("verifier config = %+v", c)
	}
}

// The JWKS copy on disk lets a restart verify tokens while NICE is down.
func TestRestartWhileNICEIsDownUsesTheJWKSCopy(t *testing.T) {
	jwks := testJWKS(t)
	nice := newFakeNICE(t)
	file := filepath.Join(t.TempDir(), "data", "jwks.json")
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Issuer: nice.URL, JWKSFile: file}
	log := slog.New(slog.DiscardHandler)

	if _, err := verify.New(VerifierConfig(cfg, devicememory.NewRepository(), log)); err != nil {
		t.Fatal(err)
	}
	if saved, err := os.ReadFile(file); err != nil || !bytes.Equal(saved, jwks) {
		t.Fatalf("JWKS copy = %q, %v; want the JWKS of NICE", saved, err)
	}

	nice.Close()
	store := devicememory.NewRepository()
	v, err := verify.New(VerifierConfig(cfg, store, log))
	if err != nil {
		t.Fatal(err)
	}
	e := &testEnv{t: t, store: store, handler: newHandler(v, store, nil)}
	cl := claims(orgA, domain.PermRead)
	cl["iss"] = nice.URL
	if r := e.do(http.MethodGet, "/devices", sign(t, cl), nil); r.Status != http.StatusOK {
		t.Fatalf("after a restart with NICE down = %d %s, want 200", r.Status, r.Body)
	}
}

func TestFeedRefusesTokensIssuedBeforeARevocation(t *testing.T) {
	nice := newFakeNICE(t)
	revokedAt := time.Now().Add(-5 * time.Second)
	nice.changes = []map[string]string{{"seq": "7", "kind": "binding_deleted", "key": verify.FeedKey("p", "usr_test", orgA),
		"org_key": verify.FeedKey("o", orgA), "occurred_at": revokedAt.UTC().Format(time.RFC3339)}}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	e := nice.serviceEnv(ctx, t)

	before := nice.claims(orgA, domain.PermRead)
	before["iat"] = revokedAt.Add(-5 * time.Second).Unix()
	wantError(t, e.do(http.MethodGet, "/devices", sign(t, before), nil), http.StatusUnauthorized, "invalid_token", "token_stale")
	if r := e.do(http.MethodGet, "/devices", sign(t, nice.claims(orgA, domain.PermRead)), nil); r.Status != http.StatusOK {
		t.Fatalf("a token issued after the revocation = %d %s, want 200", r.Status, r.Body)
	}
	// The feed names changes by keys only, so it is read without a credential.
	if got := nice.authorizations(); !slices.Contains(got, "GET /api/v1/access-changes ") {
		t.Fatalf("NICE saw %v, want the feed read without Authorization", got)
	}
}

func TestOnlineCheckAsksNICE(t *testing.T) {
	nice := newFakeNICE(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	e := nice.serviceEnv(ctx, t)
	it := e.create(orgA, "Sensor")
	tok := sign(t, nice.claims(orgA, allPerms...))

	wantError(t, e.do(http.MethodDelete, "/devices/"+it.ID, tok, nil), http.StatusForbidden, "insufficient_scope", "access_denied")
	if _, ok := e.get(orgA, it.ID); !ok {
		t.Fatal("the device was deleted although NICE refused")
	}
	nice.setAllowed(true)
	if r := e.do(http.MethodDelete, "/devices/"+it.ID, tok, nil); r.Status != http.StatusNoContent {
		t.Fatalf("delete allowed by NICE = %d %s", r.Status, r.Body)
	}
	// NICE reads the caller from the forwarded token: the body names the permission only.
	want := `{"permission":"` + domain.PermDelete + `"}`
	if got := nice.checks(); len(got) != 2 || got[0] != want {
		t.Fatalf("access checks = %v, want two of %s", got, want)
	}
	if got := nice.authorizations(); !slices.Contains(got, "POST /api/v1/access-checks Bearer "+tok) {
		t.Fatalf("NICE saw %v, want the check made with the token of the request", got)
	}
}

// A caller with more permissions than a token carries gets perms_ref in place of perms: the service asks NICE once
// for the list the reference names, and then decides alone.
func TestAPermissionReferenceIsResolvedAtNICEOnce(t *testing.T) {
	nice := newFakeNICE(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	e := nice.serviceEnv(ctx, t)
	nice.setPermissions(allPerms)
	byRef := func() string {
		c := nice.claims(orgA)
		delete(c, "perms")
		c["perms_ref"], c["jti"] = verify.PermsRef(allPerms), time.Now().Format(time.RFC3339Nano)
		return sign(t, c)
	}

	it := e.create(orgA, "Sensor")
	first := byRef()
	for _, tok := range []string{first, byRef()} {
		if r := e.do(http.MethodGet, "/devices", tok, nil); r.Status != http.StatusOK {
			t.Fatalf("list with a token by reference = %d %s, want 200", r.Status, r.Body)
		}
	}
	if r := e.do(http.MethodGet, "/devices/"+it.ID, byRef(), nil); r.Status != http.StatusOK {
		t.Fatalf("get with a token by reference = %d %s, want 200", r.Status, r.Body)
	}
	// NICE takes the principal, the context and perms_ref from the forwarded token: the body is empty.
	if got := nice.resolutions(); len(got) != 1 || got[0] != "{}" {
		t.Fatalf("resolutions = %v, want one of {}", got)
	}
	if got := nice.authorizations(); !slices.Contains(got, "POST /api/v1/permission-resolutions Bearer "+first) {
		t.Fatalf("NICE saw %v, want the resolution made with the token of the request", got)
	}

	// The permissions changed since the token was issued: NICE answers that the reference no longer matches.
	nice.setPermissions([]string{domain.PermRead})
	changed := nice.claims(orgA)
	delete(changed, "perms")
	changed["perms_ref"], changed["jti"] = verify.PermsRef([]string{domain.PermRead, domain.PermEdit}), "changed"
	wantError(t, e.do(http.MethodGet, "/devices", sign(t, changed), nil), http.StatusUnauthorized, "invalid_token", "token_stale")
}

// NICE refuses a forwarded token it does not accept (revoked, expired) with 401: the service fails closed.
func TestANICEThatRefusesTheForwardedTokenFailsClosed(t *testing.T) {
	nice := newFakeNICE(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	e := nice.serviceEnv(ctx, t)
	nice.setRefuseTokens(true)
	it := e.create(orgA, "Sensor")
	wantError(t, e.do(http.MethodDelete, "/devices/"+it.ID, sign(t, nice.claims(orgA, allPerms...)), nil),
		http.StatusServiceUnavailable, "temporarily_unavailable", "access_check_unavailable")
	if _, ok := e.get(orgA, it.ID); !ok {
		t.Fatal("the device was deleted although NICE refused the token")
	}
	c := nice.claims(orgA)
	delete(c, "perms")
	c["perms_ref"] = verify.PermsRef(allPerms)
	if r := e.do(http.MethodGet, "/devices", sign(t, c), nil); r.Status != http.StatusServiceUnavailable {
		t.Fatalf("a token by reference NICE refuses = %d %s, want 503", r.Status, r.Body)
	}
}

// fakeNICE answers the four NICE endpoints a nice_token service calls: the
// JWKS, the access change feed, the online access check and the permission resolution.
// Like NICE, it authorizes the check and the resolution by the Bearer token they forward.
type fakeNICE struct {
	*httptest.Server
	jwks    []byte
	changes []map[string]string

	mu      sync.Mutex
	allowed bool
	// refuseTokens answers 401 to the check and the resolution, as NICE does for a token it no longer accepts.
	refuseTokens bool
	seen         []string
	checked      []string
	// perms are the permissions the principal holds now; a resolution matches when its perms_ref names them.
	perms    []string
	resolved []string
}

func newFakeNICE(t *testing.T) *fakeNICE {
	t.Helper()
	f := &fakeNICE{jwks: testJWKS(t)}
	f.Server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.Close)
	return f
}

func (f *fakeNICE) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.seen = append(f.seen, r.Method+" "+r.URL.Path+" "+r.Header.Get("Authorization"))
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch r.Method + " " + r.URL.Path {
	case "GET /oauth/keys":
		_, _ = w.Write(f.jwks)
	case "GET /api/v1/access-changes":
		if r.URL.Query().Get("since") != "" {
			_ = json.NewEncoder(w).Encode(map[string]any{"changes": f.changes, "seq": "7"})
			return
		}
		// A long poll with no change: hold until the reader gives up, as NICE does.
		<-r.Context().Done()
	case "POST /api/v1/access-checks", "POST /api/v1/permission-resolutions":
		f.serveForwarded(w, r)
	default:
		http.NotFound(w, r)
	}
}

// serveForwarded answers a check or a resolution from the forwarded token, which this NICE signed.
func (f *fakeNICE) serveForwarded(w http.ResponseWriter, r *http.Request) {
	tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	f.mu.Lock()
	refuse := f.refuseTokens
	f.mu.Unlock()
	fwd, err := payload(tok)
	if !ok || refuse || err != nil || fwd.Issuer != f.URL {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": http.StatusUnauthorized, "error_code": "unauthenticated"})
		return
	}
	switch r.URL.Path {
	case "/api/v1/access-checks":
		var body bytes.Buffer
		_, _ = body.ReadFrom(r.Body)
		f.mu.Lock()
		f.checked = append(f.checked, strings.TrimSpace(body.String()))
		allowed := f.allowed
		f.mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{"allowed": allowed})
	case "/api/v1/permission-resolutions":
		var body bytes.Buffer
		_, _ = body.ReadFrom(r.Body)
		f.mu.Lock()
		f.resolved = append(f.resolved, strings.TrimSpace(body.String()))
		perms := append([]string(nil), f.perms...)
		f.mu.Unlock()
		if verify.PermsRef(perms) != fwd.PermsRef {
			_ = json.NewEncoder(w).Encode(map[string]any{"match": false})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"match": true, "permissions": perms})
	}
}

type forwarded struct {
	Issuer   string `json:"iss"`
	PermsRef string `json:"perms_ref"`
}

// payload reads the claims of a JWT without checking its signature; the fake only needs to know what it forwarded.
func payload(tok string) (forwarded, error) {
	var c forwarded
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return c, errors.New("not a JWT")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return c, err
	}
	return c, json.Unmarshal(raw, &c)
}

// claims are those of a token this NICE issued.
func (f *fakeNICE) claims(org string, perms ...string) map[string]any {
	c := claims(org, perms...)
	c["iss"] = f.URL
	return c
}

func (f *fakeNICE) setAllowed(v bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.allowed = v
}

func (f *fakeNICE) setRefuseTokens(v bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refuseTokens = v
}

func (f *fakeNICE) authorizations() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.seen...)
}

func (f *fakeNICE) setPermissions(perms []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.perms = perms
}

func (f *fakeNICE) resolutions() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.resolved...)
}

func (f *fakeNICE) checks() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.checked...)
}

// serviceEnv runs the service as run() wires it, against this NICE.
func (f *fakeNICE) serviceEnv(ctx context.Context, t *testing.T) *testEnv {
	t.Helper()
	cfg := config.Config{Issuer: f.URL, JWKSFile: filepath.Join(t.TempDir(), "jwks.json")}
	log := slog.New(slog.DiscardHandler)
	store := devicememory.NewRepository()
	v, err := verify.New(VerifierConfig(cfg, store, log))
	if err != nil {
		t.Fatal(err)
	}
	if err := StartFeed(ctx, v); err != nil {
		t.Fatal(err)
	}
	return &testEnv{t: t, store: store, handler: newHandler(v, store, nil)}
}
