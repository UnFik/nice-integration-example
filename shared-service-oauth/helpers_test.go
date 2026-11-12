package main

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

	"github.com/nice-platform/nice/pkg/nicetoken"
)

const (
	testIssuer = "http://nice.test"
	testKID    = "test-key"
	testOrigin = "http://localhost:5180"
	orgA       = "org_aaaaaaaa"
	orgB       = "org_bbbbbbbb"
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
		"iss": testIssuer, "sub": "usr_test", "aud": serviceSlug,
		"iat": now.Unix(), "nbf": now.Unix(), "exp": now.Add(30 * time.Minute).Unix(),
		"nice_org_id": org, "nice_principal_type": "user", "prj": "prj_test", "perms": perms, "pv": 1,
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

var allPerms = []string{permRead, permCreate, permEdit, permDelete}

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

type testEnv struct {
	t       *testing.T
	store   *Store
	handler http.Handler
}

// newTestEnv builds the handler with the production wiring of verifierConfig,
// with the test key set instead of NICE and checker as the online checker.
func newTestEnv(t *testing.T, checker nicetoken.AccessChecker) *testEnv {
	t.Helper()
	cfg := Config{Issuer: testIssuer, CORSOrigins: []string{testOrigin}}
	store := NewStore()
	vc := verifierConfig(cfg, store, slog.New(slog.DiscardHandler))
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

// create makes an item for org directly in the store, bypassing the API.
func (e *testEnv) create(org, name string) Item {
	e.t.Helper()
	_ = e.store.CreateTenant(context.Background(), org)
	return e.store.Create(org, name, 1, "usr_test", "")
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
