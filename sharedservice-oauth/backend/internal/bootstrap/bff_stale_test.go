package bootstrap

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/ABA-Developer/nice-auth-sdk/verify"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

// staleEnv signs in with tokens issued ten seconds ago and then starts the
// access change feed with a change of the user in organization A five seconds
// ago, so the token of the active context A/P kept by niceauth/credentials is stale.
func staleEnv(t *testing.T) (*bffEnv, *browser) {
	t.Helper()
	e := newBFFEnv(t, nil)
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

func TestStaleContextTokenIsRenewedOnce(t *testing.T) {
	e, b := staleEnv(t)
	e.nice.set(func(f *bffNICE) { f.iatShift = 0 })
	before := len(e.nice.refreshScopes())

	if r := b.do(http.MethodGet, "/devices", nil); r.Status != http.StatusOK {
		t.Fatalf("devices with a stale cached token = %d %s, want 200 with a new token", r.Status, r.Body)
	}
	if n := len(e.nice.refreshScopes()) - before; n != 1 {
		t.Fatalf("%d refreshes, want one for the stale token", n)
	}
	m := b.session()
	if ce, _ := m["context_error"].(string); ce != "" || len(permsOf(m)) == 0 {
		t.Fatalf("session = %v, want the perms of the new token", m)
	}
	if n := len(e.nice.refreshScopes()) - before; n != 1 {
		t.Fatalf("%d refreshes after the session read, want the new token reused", n)
	}
}

func TestStaleContextTokenTwiceIsNotRenewedAgain(t *testing.T) {
	e, b := staleEnv(t)
	before := len(e.nice.refreshScopes())

	// NICE keeps issuing tokens older than the change: one new token, then token_stale.
	wantError(t, b.do(http.MethodGet, "/devices", nil), http.StatusUnauthorized, "invalid_token", "token_stale")
	if n := len(e.nice.refreshScopes()) - before; n != 1 {
		t.Fatalf("%d refreshes, want exactly one", n)
	}
}

func TestStaleBearerTokenIsNotRenewed(t *testing.T) {
	e, _ := staleEnv(t)
	before := len(e.nice.refreshScopes())
	cl := claims(orgA, domain.PermRead)
	cl["iss"], cl["prj"], cl["sub"] = e.nice.URL, projectP, bffUser
	cl["iat"] = time.Now().Add(-10 * time.Second).Unix()

	wantError(t, e.do(http.MethodGet, "/devices", sign(t, cl), nil), http.StatusUnauthorized, "invalid_token", "token_stale")
	if n := len(e.nice.refreshScopes()) - before; n != 0 {
		t.Fatalf("%d refreshes for a Bearer caller, want none", n)
	}
}
