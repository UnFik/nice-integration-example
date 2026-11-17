package bootstrap

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	itemmemory "github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/memory/item"
	itempostgres "github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/postgres/item"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/platform/config"
)

const unreachableDB = "postgres://inventaris:s3cret@127.0.0.1:1/inventaris?connect_timeout=2"

func TestWithoutDatabaseURLTheItemsStayInMemory(t *testing.T) {
	store, closeStore, err := OpenItemStore(context.Background(), config.Config{})
	if err != nil {
		t.Fatal(err)
	}
	defer closeStore()
	if _, ok := store.(*itemmemory.Repository); !ok {
		t.Fatalf("store = %T, want the memory repository", store)
	}
	if StoreName(config.Config{}) != "memory" {
		t.Fatalf("store name = %q", StoreName(config.Config{}))
	}
}

func TestUnreachableDatabaseStopsTheServiceWithoutPrintingTheURL(t *testing.T) {
	_, _, err := OpenItemStore(context.Background(), config.Config{DatabaseURL: unreachableDB})
	if err == nil || !strings.Contains(err.Error(), config.EnvDatabaseURL) {
		t.Fatalf("err = %v, want an error naming %s", err, config.EnvDatabaseURL)
	}
	if strings.Contains(err.Error(), "s3cret") {
		t.Fatalf("the error prints the password of %s: %v", config.EnvDatabaseURL, err)
	}

	var logs strings.Builder
	err = Run(context.Background(), env(map[string]string{
		config.EnvIssuer: testIssuer, config.EnvDatabaseURL: unreachableDB,
		config.EnvJWKSFile: filepath.Join(t.TempDir(), "jwks.json"),
	}), filepath.Join(t.TempDir(), ".env"), slog.New(slog.NewTextHandler(&logs, nil)))
	if err == nil || !strings.Contains(err.Error(), config.EnvDatabaseURL) || strings.Contains(err.Error()+logs.String(), "s3cret") {
		t.Fatalf("run = %v, log %q; want an error naming %s and no password", err, logs.String(), config.EnvDatabaseURL)
	}
}

// testDatabase returns TEST_DATABASE_URL or skips the test.
func testDatabase(t *testing.T) string {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	return url
}

func randomOrg(t *testing.T) string {
	t.Helper()
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return "org_" + hex.EncodeToString(b)
}

func TestDatabaseURLSelectsPostgres(t *testing.T) {
	cfg := config.Config{DatabaseURL: testDatabase(t)}
	store, closeStore, err := OpenItemStore(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer closeStore()
	if _, ok := store.(*itempostgres.Repository); !ok {
		t.Fatalf("store = %T, want the PostgreSQL repository", store)
	}
	if StoreName(cfg) != "postgres" {
		t.Fatalf("store name = %q", StoreName(cfg))
	}
}

func postgresEnv(t *testing.T, checker *fakeChecker) *testEnv {
	t.Helper()
	store, closeStore, err := OpenItemStore(context.Background(), config.Config{DatabaseURL: testDatabase(t)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeStore)
	return newTestEnvWith(t, checker, store.(*itempostgres.Repository))
}

func TestItemsInPostgresThroughTheAPI(t *testing.T) {
	e := postgresEnv(t, &fakeChecker{allowed: true})
	a, b := randomOrg(t), randomOrg(t)
	if e.hasTenant(a) {
		t.Fatal("tenant exists before any request")
	}
	if r := e.do(http.MethodGet, "/items", tokenIn(t, a, projectP, domain.PermRead), nil); r.Status != http.StatusOK || strings.TrimSpace(r.Body) != `{"items":[]}` {
		t.Fatalf("first list = %d %s", r.Status, r.Body)
	}
	if !e.hasTenant(a) {
		t.Fatal("the first request did not create the tenant")
	}

	ap := tokenIn(t, a, projectP, allPerms...)
	r := e.do(http.MethodPost, "/items", ap, map[string]any{"name": "  Kabel  ", "quantity": 3})
	if r.Status != http.StatusCreated {
		t.Fatalf("create = %d %s", r.Status, r.Body)
	}
	created := r.JSON(t)
	id := created["id"].(string)
	if !strings.HasPrefix(id, "itm_") || created["name"] != "Kabel" || created["created_by"] != "usr_test" {
		t.Fatalf("created = %v", created)
	}
	if r = e.do(http.MethodPut, "/items/"+id, ap, map[string]any{"name": "Kabel Cat6", "quantity": 0}); r.Status != http.StatusOK || r.JSON(t)["quantity"] != float64(0) {
		t.Fatalf("update = %d %s", r.Status, r.Body)
	}
	for name, tok := range map[string]string{
		"project Q of the same organization": tokenIn(t, a, projectQ, allPerms...),
		"another organization":               tokenIn(t, b, projectP, allPerms...),
	} {
		if r = e.do(http.MethodGet, "/items", tok, nil); strings.Contains(r.Body, id) {
			t.Errorf("%s lists the item: %s", name, r.Body)
		}
		for _, m := range []string{http.MethodGet, http.MethodDelete} {
			if r = e.do(m, "/items/"+id, tok, nil); r.Status != http.StatusNotFound {
				t.Errorf("%s: %s = %d, want 404", name, m, r.Status)
			}
		}
	}
	if r = e.do(http.MethodGet, "/items/itm_not-hex", ap, nil); r.Status != http.StatusNotFound {
		t.Fatalf("malformed id = %d, want 404", r.Status)
	}
	if r = e.do(http.MethodDelete, "/items/"+id, ap, nil); r.Status != http.StatusNoContent {
		t.Fatalf("delete = %d %s", r.Status, r.Body)
	}
	if r = e.do(http.MethodGet, "/items/"+id, ap, nil); r.Status != http.StatusNotFound {
		t.Fatalf("get after delete = %d", r.Status)
	}
}

func TestFirstRequestsOfAnOrganizationAtOnceInPostgres(t *testing.T) {
	e := postgresEnv(t, nil)
	org := randomOrg(t)
	tok := tokenIn(t, org, testProject, allPerms...)
	var wg sync.WaitGroup
	status := make([]int, 8)
	for i := range status {
		wg.Go(func() {
			status[i] = e.do(http.MethodPost, "/items", tok, map[string]any{"name": "Kabel", "quantity": 1}).Status
		})
	}
	wg.Wait()
	for i, s := range status {
		if s != http.StatusCreated {
			t.Errorf("request %d = %d, want 201", i, s)
		}
	}
	if items := e.list(org); len(items) != len(status) {
		t.Fatalf("items = %d, want %d", len(items), len(status))
	}
}
