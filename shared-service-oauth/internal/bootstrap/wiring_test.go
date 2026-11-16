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

	itemmemory "github.com/nice-platform/example/shared-service-oauth/internal/adapter/memory/item"
	"github.com/nice-platform/example/shared-service-oauth/internal/domain"
	"github.com/nice-platform/example/shared-service-oauth/internal/platform/config"
	"github.com/nice-platform/nice/pkg/nicetoken"
)

func TestCheckerOnlyWithTheServiceKey(t *testing.T) {
	store := itemmemory.NewRepository()
	log := slog.New(slog.DiscardHandler)
	if c := VerifierConfig(config.Config{Issuer: testIssuer}, store, log); c.Checker != nil {
		t.Fatalf("checker without %s = %T, want nil", config.EnvServiceAPIKey, c.Checker)
	}
	c := VerifierConfig(config.Config{Issuer: testIssuer, ServiceAPIKey: "nice_sk_test.secret"}, store, log)
	if _, ok := c.Checker.(*nicetoken.HTTPChecker); !ok {
		t.Fatalf("checker with %s = %T, want *nicetoken.HTTPChecker", config.EnvServiceAPIKey, c.Checker)
	}
	if c.Service != domain.ServiceSlug || c.Tenants == nil {
		t.Fatalf("verifier config = %+v", c)
	}
}

func TestWithoutTheServiceKeyTheServiceWarnsAndStillServes(t *testing.T) {
	var logs bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logs, nil))
	cfg := config.Config{Issuer: testIssuer, CORSOrigins: []string{testOrigin}}
	store := itemmemory.NewRepository()
	vc := VerifierConfig(cfg, store, log)
	vc.StaticJWKS = testJWKS(t)
	v, err := nicetoken.New(vc)
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
	if r := e.do(http.MethodGet, "/items", token(t, orgA, domain.PermRead), nil); r.Status != http.StatusOK {
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

	if _, err := nicetoken.New(VerifierConfig(cfg, itemmemory.NewRepository(), log)); err != nil {
		t.Fatal(err)
	}
	if saved, err := os.ReadFile(file); err != nil || !bytes.Equal(saved, jwks) {
		t.Fatalf("JWKS copy = %q, %v; want the JWKS of NICE", saved, err)
	}

	nice.Close()
	store := itemmemory.NewRepository()
	v, err := nicetoken.New(VerifierConfig(cfg, store, log))
	if err != nil {
		t.Fatal(err)
	}
	e := &testEnv{t: t, store: store, handler: newHandler(v, store, nil)}
	cl := claims(orgA, domain.PermRead)
	cl["iss"] = nice.URL
	if r := e.do(http.MethodGet, "/items", sign(t, cl), nil); r.Status != http.StatusOK {
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
	wantError(t, e.do(http.MethodGet, "/items", sign(t, before), nil), http.StatusUnauthorized, "invalid_token", "token_stale")
	if r := e.do(http.MethodGet, "/items", sign(t, nice.claims(orgA, domain.PermRead)), nil); r.Status != http.StatusOK {
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
	it := e.create(orgA, "Kabel")
	tok := sign(t, nice.claims(orgA, allPerms...))

	wantError(t, e.do(http.MethodDelete, "/items/"+it.ID, tok, nil), http.StatusForbidden, "insufficient_scope", "access_denied")
	if _, ok := e.store.Get(orgA, it.ID); !ok {
		t.Fatal("the item was deleted although NICE refused")
	}
	nice.setAllowed(true)
	if r := e.do(http.MethodDelete, "/items/"+it.ID, tok, nil); r.Status != http.StatusNoContent {
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

const fakeServiceKey = "nice_sk_test.secret"

// fakeNICE answers the three NICE endpoints a nice_token service calls: the
// JWKS, the access change feed and the online access check.
type fakeNICE struct {
	*httptest.Server
	jwks    []byte
	changes []map[string]string

	mu      sync.Mutex
	allowed bool
	seen    []string
	checked []string
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
	store := itemmemory.NewRepository()
	v, err := nicetoken.New(VerifierConfig(cfg, store, log))
	if err != nil {
		t.Fatal(err)
	}
	if err := StartFeed(ctx, v, cfg, log); err != nil {
		t.Fatal(err)
	}
	return &testEnv{t: t, store: store, handler: newHandler(v, store, nil)}
}
