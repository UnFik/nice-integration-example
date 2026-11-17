package bootstrap

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"

	itemmemory "github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/memory/item"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest"
	itemusecase "github.com/nice-platform/example/sharedservice-oauth/backend/internal/app/item/usecase"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/platform/config"
	"github.com/nice-platform/nice/pkg/nicetoken"
)

const (
	testIssuer = "http://nice.test"
	testKID    = "test-key"
	testOrigin = "http://localhost:5180"
	orgA       = "org_aaaaaaaa"
	orgB       = "org_bbbbbbbb"
	// testProject is the prj of claims.
	testProject = "prj_test"
)

var testKey = sync.OnceValue(func() *rsa.PrivateKey {
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	return k
})

func testJWKS(t *testing.T) []byte {
	t.Helper()
	set := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &testKey().PublicKey, KeyID: testKID, Algorithm: "RS256", Use: "sig"}}}
	b, err := json.Marshal(set)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// claims returns the claims NICE puts in a user token for this service.
func claims(org string, perms ...string) map[string]any {
	now := time.Now()
	return map[string]any{
		"iss": testIssuer, "sub": "usr_test", "aud": domain.ServiceSlug,
		"iat": now.Unix(), "nbf": now.Unix(), "exp": now.Add(30 * time.Minute).Unix(),
		"nice_org_id": org, "nice_principal_type": "user", "prj": testProject, "perms": perms, "pv": 1,
	}
}

func sign(t *testing.T, c map[string]any) string {
	t.Helper()
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: jose.JSONWebKey{Key: testKey(), KeyID: testKID}},
		(&jose.SignerOptions{}).WithType("JWT"))
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	jws, err := signer.Sign(payload)
	if err != nil {
		t.Fatal(err)
	}
	token, err := jws.CompactSerialize()
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func token(t *testing.T, org string, perms ...string) string { return sign(t, claims(org, perms...)) }

var allPerms = []string{domain.PermRead, domain.PermCreate, domain.PermEdit, domain.PermDelete}

type fakeChecker struct {
	mu      sync.Mutex
	allowed bool
	err     error
	calls   []string
}

func (f *fakeChecker) Check(_ context.Context, c *nicetoken.Claims, permission string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, c.Subject+" "+c.OrgID+" "+permission)
	return f.allowed, f.err
}

func (f *fakeChecker) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

// testStore is what the tests read back from either item store.
type testStore interface {
	ItemStore
	HasTenant(ctx context.Context, org string) (bool, error)
}

type testEnv struct {
	t       *testing.T
	store   testStore
	handler http.Handler
}

// newTestEnv builds the handler with the production wiring of verifierConfig,
// with the test key set instead of NICE and checker as the online checker.
func newTestEnv(t *testing.T, checker nicetoken.AccessChecker) *testEnv {
	t.Helper()
	return newTestEnvWith(t, checker, itemmemory.NewRepository())
}

func newTestEnvWith(t *testing.T, checker nicetoken.AccessChecker, store testStore) *testEnv {
	t.Helper()
	cfg := config.Config{Issuer: testIssuer, CORSOrigins: []string{testOrigin}}
	vc := VerifierConfig(cfg, store, slog.New(slog.DiscardHandler))
	vc.StaticJWKS = testJWKS(t)
	if checker != nil {
		vc.Checker = checker
	}
	v, err := nicetoken.New(vc)
	if err != nil {
		t.Fatal(err)
	}
	return &testEnv{t: t, store: store, handler: newHandler(v, store, cfg.CORSOrigins)}
}

type response struct {
	Status int
	Header http.Header
	Body   string
}

func (r response) JSON(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(r.Body), &m); err != nil {
		t.Fatalf("body is not a JSON object: %v: %s", err, r.Body)
	}
	return m
}

func (e *testEnv) do(method, path, token string, body any, headers ...string) response {
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
	return response{Status: rec.Code, Header: rec.Header(), Body: rec.Body.String()}
}

// create makes an item for org and testProject directly in the store, bypassing the API.
func (e *testEnv) create(org, name string) domain.Item {
	e.t.Helper()
	it, err := e.store.Create(context.Background(), scopeOf(org, testProject), domain.ItemFields{Name: name, Quantity: 1})
	if err != nil {
		e.t.Fatal(err)
	}
	return it
}

// scopeOf is the scope of a token of claims(org) with prj set to project.
func scopeOf(org, project string) domain.Scope {
	return domain.Scope{OrgID: org, ProjectID: project, Subject: "usr_test"}
}

func (e *testEnv) list(org string) []domain.Item {
	e.t.Helper()
	items, err := e.store.List(context.Background(), scopeOf(org, testProject))
	if err != nil {
		e.t.Fatal(err)
	}
	return items
}

func (e *testEnv) get(org, id string) (domain.Item, bool) {
	it, err := e.store.Get(context.Background(), scopeOf(org, testProject), id)
	return it, err == nil
}

func (e *testEnv) hasTenant(org string) bool {
	e.t.Helper()
	ok, err := e.store.HasTenant(context.Background(), org)
	if err != nil {
		e.t.Fatal(err)
	}
	return ok
}

func wantError(t *testing.T, r response, status int, code, description string) {
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

func newHandler(v *nicetoken.Verifier, repo itemusecase.Repo, origins []string) http.Handler {
	return rest.NewHandler(v, &itemusecase.Service{Repo: repo}, origins)
}
