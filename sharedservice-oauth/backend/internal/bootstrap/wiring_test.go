package bootstrap

import (
	"bytes"
	"context"
	"encoding/json"
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

func TestCheckerAndResolverOnlyWithTheServiceKey(t *testing.T) {
	store := devicememory.NewRepository()
	log := slog.New(slog.DiscardHandler)
	if c := VerifierConfig(config.Config{Issuer: testIssuer}, store, log); c.Checker != nil || c.Resolver != nil {
		t.Fatalf("without %s: checker %T, resolver %T, want neither", config.EnvServiceAPIKey, c.Checker, c.Resolver)
	}
	c := VerifierConfig(config.Config{Issuer: testIssuer, ServiceAPIKey: "nice_sk_test.secret"}, store, log)
	if _, ok := c.Checker.(*verify.HTTPChecker); !ok {
		t.Fatalf("checker with %s = %T, want *verify.HTTPChecker", config.EnvServiceAPIKey, c.Checker)
	}
	if _, ok := c.Resolver.(*verify.HTTPResolver); !ok {
		t.Fatalf("resolver with %s = %T, want *verify.HTTPResolver", config.EnvServiceAPIKey, c.Resolver)
	}
	if c.Service != domain.ServiceSlug || c.Tenants == nil {
		t.Fatalf("verifier config = %+v", c)
	}
}

func TestWithoutTheServiceKeyTheServiceWarnsAndStillServes(t *testing.T) {
	var logs bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logs, nil))
	cfg := config.Config{Issuer: testIssuer, CORSOrigins: []string{testOrigin}}
	store := devicememory.NewRepository()
	vc := VerifierConfig(cfg, store, log)
	vc.StaticJWKS = testJWKS(t)
	v, err := verify.New(vc)
	if err != nil {
		t.Fatal(err)
	}
	if err := StartFeed(context.Background(), v, cfg, log); err != nil {
		t.Fatal(err)
	}
	if out := logs.String(); !strings.Contains(out, "level=WARN") || !strings.Contains(out, config.EnvServiceAPIKey) {
		t.Fatalf("log = %q, want a warning naming %s", out, config.EnvServiceAPIKey)
	}
	e := &testEnv{t: t, store: store, handler: newHandler(v, store, cfg.CORSOrigins)}
	if r := e.do(http.MethodGet, "/devices", token(t, orgA, domain.PermRead), nil); r.Status != http.StatusOK {
		t.Fatalf("list without the feed = %d %s", r.Status, r.Body)
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
	nice.changes = []map[string]string{{"seq": "7", "organization_id": orgA, "kind": "binding_deleted",
		"principal_id": "usr_test", "occurred_at": revokedAt.UTC().Format(time.RFC3339)}}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	e := nice.serviceEnv(ctx, t)

	before := nice.claims(orgA, domain.PermRead)
	before["iat"] = revokedAt.Add(-5 * time.Second).Unix()
	wantError(t, e.do(http.MethodGet, "/devices", sign(t, before), nil), http.StatusUnauthorized, "invalid_token", "token_stale")
	if r := e.do(http.MethodGet, "/devices", sign(t, nice.claims(orgA, domain.PermRead)), nil); r.Status != http.StatusOK {
		t.Fatalf("a token issued after the revocation = %d %s, want 200", r.Status, r.Body)
	}
	if got := nice.authorizations(); !slices.Contains(got, "GET /api/v1/access-changes ApiKey "+fakeServiceKey) {
		t.Fatalf("NICE saw %v, want the feed read with the nice.service key", got)
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
	want := `{"principal_type":"PRINCIPAL_TYPE_USER","principal_id":"usr_test","organization_id":"` + orgA +
		`","project_id":"prj_test","permission":"` + domain.PermDelete + `"}`
	if got := nice.checks(); len(got) != 2 || got[0] != want {
		t.Fatalf("access checks = %v, want two of %s", got, want)
	}
	if got := nice.authorizations(); !slices.Contains(got, "POST /api/v1/access-checks ApiKey "+fakeServiceKey) {
		t.Fatalf("NICE saw %v, want the check made with the nice.service key", got)
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
	for range 2 {
		if r := e.do(http.MethodGet, "/devices", byRef(), nil); r.Status != http.StatusOK {
			t.Fatalf("list with a token by reference = %d %s, want 200", r.Status, r.Body)
		}
	}
	if r := e.do(http.MethodGet, "/devices/"+it.ID, byRef(), nil); r.Status != http.StatusOK {
		t.Fatalf("get with a token by reference = %d %s, want 200", r.Status, r.Body)
	}
	want := `{"principal_type":"PRINCIPAL_TYPE_USER","principal_id":"usr_test","organization_id":"` + orgA +
		`","project_id":"prj_test","service":"` + domain.ServiceSlug + `","perms_ref":"` + verify.PermsRef(allPerms) + `"}`
	if got := nice.resolutions(); len(got) != 1 || got[0] != want {
		t.Fatalf("resolutions = %v, want one of %s", got, want)
	}
	if got := nice.authorizations(); !slices.Contains(got, "POST /api/v1/permission-resolutions ApiKey "+fakeServiceKey) {
		t.Fatalf("NICE saw %v, want the resolution made with the nice.service key", got)
	}

	// The permissions changed since the token was issued: NICE answers that the reference no longer matches.
	nice.setPermissions([]string{domain.PermRead})
	changed := nice.claims(orgA)
	delete(changed, "perms")
	changed["perms_ref"], changed["jti"] = verify.PermsRef([]string{domain.PermRead, domain.PermEdit}), "changed"
	wantError(t, e.do(http.MethodGet, "/devices", sign(t, changed), nil), http.StatusUnauthorized, "invalid_token", "token_stale")
}

func TestWithoutTheServiceKeyATokenByReferenceIsRefusedWith503(t *testing.T) {
	cfg := config.Config{Issuer: testIssuer, CORSOrigins: []string{testOrigin}}
	store := devicememory.NewRepository()
	vc := VerifierConfig(cfg, store, slog.New(slog.DiscardHandler))
	vc.StaticJWKS = testJWKS(t)
	v, err := verify.New(vc)
	if err != nil {
		t.Fatal(err)
	}
	e := &testEnv{t: t, store: store, handler: newHandler(v, store, cfg.CORSOrigins)}
	c := claims(orgA)
	delete(c, "perms")
	c["perms_ref"], c["jti"] = verify.PermsRef(allPerms), "no-key"
	if r := e.do(http.MethodGet, "/devices", sign(t, c), nil); r.Status != http.StatusServiceUnavailable {
		t.Fatalf("a token by reference without %s = %d %s, want 503", config.EnvServiceAPIKey, r.Status, r.Body)
	}
}

const fakeServiceKey = "nice_sk_test.secret"

// fakeNICE answers the four NICE endpoints a nice_token service calls: the
// JWKS, the access change feed, the online access check and the permission resolution.
type fakeNICE struct {
	*httptest.Server
	jwks    []byte
	changes []map[string]string

	mu      sync.Mutex
	allowed bool
	seen    []string
	checked []string
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
	case "POST /api/v1/access-checks":
		var body bytes.Buffer
		_, _ = body.ReadFrom(r.Body)
		f.mu.Lock()
		f.checked = append(f.checked, strings.TrimSpace(body.String()))
		allowed := f.allowed
		f.mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{"allowed": allowed})
	case "POST /api/v1/permission-resolutions":
		var body bytes.Buffer
		_, _ = body.ReadFrom(r.Body)
		var req struct {
			PermsRef string `json:"perms_ref"`
		}
		_ = json.Unmarshal(body.Bytes(), &req)
		f.mu.Lock()
		f.resolved = append(f.resolved, strings.TrimSpace(body.String()))
		perms := append([]string(nil), f.perms...)
		f.mu.Unlock()
		if verify.PermsRef(perms) != req.PermsRef {
			_ = json.NewEncoder(w).Encode(map[string]any{"match": false})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"match": true, "permissions": perms})
	default:
		http.NotFound(w, r)
	}
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

// serviceEnv runs the service as run() wires it, against this NICE and with the nice.service key set.
func (f *fakeNICE) serviceEnv(ctx context.Context, t *testing.T) *testEnv {
	t.Helper()
	cfg := config.Config{Issuer: f.URL, JWKSFile: filepath.Join(t.TempDir(), "jwks.json"), ServiceAPIKey: fakeServiceKey}
	log := slog.New(slog.DiscardHandler)
	store := devicememory.NewRepository()
	v, err := verify.New(VerifierConfig(cfg, store, log))
	if err != nil {
		t.Fatal(err)
	}
	if err := StartFeed(ctx, v, cfg, log); err != nil {
		t.Fatal(err)
	}
	return &testEnv{t: t, store: store, handler: newHandler(v, store, nil)}
}
