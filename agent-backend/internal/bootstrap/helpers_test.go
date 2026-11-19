package bootstrap

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	mcpadapter "github.com/nice-platform/example/agent-backend/internal/adapter/mcp"
	"github.com/nice-platform/example/agent-backend/internal/domain"
	"github.com/nice-platform/example/agent-backend/internal/platform/config"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/ABA-Developer/nice-auth-sdk/verify"
)

const (
	testKID      = "test-key"
	testClientID = "agen-backend-test"
	testSecret   = "agen-backend-secret-test"
	testUser     = "usr_test"
	orgA         = "org_aaaaaaaa"
	orgB         = "org_bbbbbbbb"
	orgC         = "org_cccccccc"
	prjP         = "prj_pppppppp"
	prjQ         = "prj_qqqqqqqq"
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

func sign(t testing.TB, c map[string]any) string {
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
	tok, err := jws.CompactSerialize()
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

// userClaims are the claims NICE puts in a user token of the client agen-user for aud.
func userClaims(issuer, aud, org, prj string, perms ...string) map[string]any {
	now := time.Now()
	return map[string]any{
		"iss": issuer, "sub": testUser, "aud": aud, "azp": "agen-user",
		"iat": now.Unix(), "nbf": now.Unix(), "exp": now.Add(30 * time.Minute).Unix(),
		"nice_org_id": org, "nice_principal_type": "user", "prj": prj, "perms": perms, "pv": 1,
	}
}

func readClaims(t testing.TB, token string) map[string]any {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("not a JWT")
	}
	var c map[string]any
	raw, err := jose.ParseSigned(token, []jose.SignatureAlgorithm{jose.RS256})
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw.UnsafePayloadWithoutVerification(), &c); err != nil {
		t.Fatal(err)
	}
	return c
}

type testEnv struct {
	t      *testing.T
	cfg    config.Config
	nice   *fakeNICE
	inv    *fakeService
	agent  *httptest.Server
	issuer string
}

// newTestEnv runs the agent with the production wiring of NewCallService and NewHandler, in front of a fake NICE (token
// endpoint and JWKS) and a fake inventaris.
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	e := &testEnv{t: t}
	e.nice = newFakeNICE(t)
	e.issuer = e.nice.URL()
	e.inv = newFakeService(t, e.issuer, "inventaris")
	e.cfg = config.Config{
		Issuer: e.issuer, ClientID: testClientID, ClientSecret: testSecret,
		InventarisURL: e.inv.srv.URL, PublicURL: "http://agent.test",
	}
	log := slog.New(slog.DiscardHandler)
	vc := VerifierConfig(e.cfg, log)
	vc.StaticJWKS = testJWKS(t)
	v, err := verify.New(vc)
	if err != nil {
		t.Fatal(err)
	}
	calls, err := NewCallService(e.cfg, http.DefaultClient, log)
	if err != nil {
		t.Fatal(err)
	}
	e.agent = httptest.NewServer(NewHandler(v, calls, e.cfg))
	t.Cleanup(e.agent.Close)
	return e
}

// userToken is a token of the agent for the user in org and prj, with agen.mcp.use.
func (e *testEnv) userToken(org, prj string) string {
	return sign(e.t, userClaims(e.issuer, domain.ServiceSlug, org, prj, domain.PermUse))
}

func (e *testEnv) post(path, token, body string) *http.Response {
	e.t.Helper()
	req, err := http.NewRequest(http.MethodPost, e.agent.URL+path, strings.NewReader(body))
	if err != nil {
		e.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	e.t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

type bearerTransport struct{ token string }

func (b bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}

// connect opens an MCP session to the agent with token, the way an MCP client does over Streamable HTTP.
func (e *testEnv) connect(token string) *mcp.ClientSession {
	e.t.Helper()
	c := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0"}, nil)
	s, err := c.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint: e.agent.URL + mcpadapter.MCPPath, HTTPClient: &http.Client{Transport: bearerTransport{token}},
	}, nil)
	if err != nil {
		e.t.Fatal(err)
	}
	e.t.Cleanup(func() { _ = s.Close() })
	return s
}
