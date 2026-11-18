package devicepostgres_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	devicepostgres "github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/postgres/device"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/platform/postgres"
	"github.com/nice-platform/example/sharedservice-oauth/backend/migrations"
)

// open connects to TEST_DATABASE_URL and migrates it, or skips the test.
// Every test uses organizations of its own, so tests share the database.
func open(t *testing.T) (*devicepostgres.Repository, *pgxpool.Pool) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := postgres.Migrate(ctx, pool, migrations.FS); err != nil {
		t.Fatal(err)
	}
	return devicepostgres.NewRepository(pool), pool
}

func randomOrg() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return "org_" + hex.EncodeToString(b)
}

func fields(name string) domain.DeviceFields {
	return domain.DeviceFields{Name: name, SerialNumber: "SN-1", Status: domain.StatusActive}
}

func TestEachScopeSeesOnlyItsDevices(t *testing.T) {
	repo, _ := open(t)
	ctx := context.Background()
	a, b := randomOrg(), randomOrg()
	scopes := []domain.Scope{
		{OrgID: a, ProjectID: "prj_p", Subject: "usr_1"},
		{OrgID: a, ProjectID: "prj_q", Subject: "usr_1"},
		{OrgID: b, ProjectID: "prj_r", Subject: "usr_2"},
		// Same project ID under another organization: still another scope.
		{OrgID: b, ProjectID: "prj_p", Subject: "usr_2"},
	}
	created := make([]domain.Device, len(scopes))
	for i, sc := range scopes {
		it, err := repo.Create(ctx, sc, fields("Sensor "+sc.OrgID+" "+sc.ProjectID))
		if err != nil {
			t.Fatal(err)
		}
		created[i] = it
	}
	for i, sc := range scopes {
		devices, err := repo.List(ctx, sc)
		if err != nil {
			t.Fatal(err)
		}
		if len(devices) != 1 || devices[0].ID != created[i].ID {
			t.Errorf("list of %+v = %+v, want only %s", sc, devices, created[i].ID)
		}
		for j, it := range created {
			if i == j {
				continue
			}
			if _, err := repo.Get(ctx, sc, it.ID); !errors.Is(err, domain.ErrDeviceNotFound) {
				t.Errorf("%+v get of the device of %+v: %v, want not found", sc, scopes[j], err)
			}
			if _, err := repo.Update(ctx, sc, it.ID, fields("Diambil")); !errors.Is(err, domain.ErrDeviceNotFound) {
				t.Errorf("%+v update of the device of %+v: %v, want not found", sc, scopes[j], err)
			}
			if err := repo.Delete(ctx, sc, it.ID); !errors.Is(err, domain.ErrDeviceNotFound) {
				t.Errorf("%+v delete of the device of %+v: %v, want not found", sc, scopes[j], err)
			}
		}
	}
	for i, sc := range scopes {
		if got, err := repo.Get(ctx, sc, created[i].ID); err != nil || got != created[i] {
			t.Errorf("device of %+v = %+v, %v; want %+v unchanged", sc, got, err, created[i])
		}
	}
}

func TestRowHoldsTheScopeOfTheToken(t *testing.T) {
	repo, pool := open(t)
	ctx := context.Background()
	sc := domain.Scope{OrgID: randomOrg(), ProjectID: "prj_p", Subject: "usr_1", Actor: "cli_agent"}
	it, err := repo.Create(ctx, sc, fields("Sensor"))
	if err != nil {
		t.Fatal(err)
	}
	var org, project, createdBy string
	var actedBy *string
	err = pool.QueryRow(ctx, "SELECT org_id, project_id, created_by, acted_by FROM devices WHERE org_id = $1", sc.OrgID).
		Scan(&org, &project, &createdBy, &actedBy)
	if err != nil {
		t.Fatal(err)
	}
	if org != sc.OrgID || project != sc.ProjectID || createdBy != "usr_1" || actedBy == nil || *actedBy != "cli_agent" {
		t.Fatalf("row = %s %s %s %v", org, project, createdBy, actedBy)
	}
	if it.OrgID != sc.OrgID || it.ProjectID != sc.ProjectID || it.CreatedBy != "usr_1" || it.ActedBy != "cli_agent" {
		t.Fatalf("device = %+v", it)
	}

	sc.Actor = ""
	if _, err := repo.Create(ctx, sc, fields("Tanpa agen")); err != nil {
		t.Fatal(err)
	}
	var nulls int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM devices WHERE org_id = $1 AND acted_by IS NULL", sc.OrgID).Scan(&nulls); err != nil || nulls != 1 {
		t.Fatalf("devices without act stored with acted_by NULL = %d, %v; want 1", nulls, err)
	}
}

func TestLifecycleAndOrder(t *testing.T) {
	repo, _ := open(t)
	ctx := context.Background()
	sc := domain.Scope{OrgID: randomOrg(), ProjectID: "prj_p", Subject: "usr_1"}
	if devices, err := repo.List(ctx, sc); err != nil || devices == nil || len(devices) != 0 {
		t.Fatalf("empty list = %#v, %v; want an empty, non-nil list", devices, err)
	}
	first, _ := repo.Create(ctx, sc, fields("Pertama"))
	second, _ := repo.Create(ctx, sc, fields("Kedua"))
	updated, err := repo.Update(ctx, sc, first.ID, domain.DeviceFields{Name: "Pertama baru", SerialNumber: "SN-7", Status: domain.StatusMaintenance, Location: "Gudang B"})
	if err != nil || updated.Name != "Pertama baru" || updated.SerialNumber != "SN-7" || updated.Status != domain.StatusMaintenance || updated.Location != "Gudang B" || updated.CreateTime != first.CreateTime {
		t.Fatalf("update = %+v, %v", updated, err)
	}
	devices, err := repo.List(ctx, sc)
	if err != nil || len(devices) != 2 || devices[0].ID != first.ID || devices[1].ID != second.ID {
		t.Fatalf("list = %+v, %v; want oldest first", devices, err)
	}
	if err := repo.Delete(ctx, sc, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Get(ctx, sc, first.ID); !errors.Is(err, domain.ErrDeviceNotFound) {
		t.Fatalf("get after delete = %v", err)
	}
	for _, id := range []string{"", "dev_", "dev_zz", "abc", second.ID[4:], "dev_" + second.ID[4:] + "00"} {
		if _, err := repo.Get(ctx, sc, id); !errors.Is(err, domain.ErrDeviceNotFound) {
			t.Errorf("get %q = %v, want not found", id, err)
		}
	}
}

func TestFirstTenantAtOnceMakesOneRow(t *testing.T) {
	repo, pool := open(t)
	ctx := context.Background()
	org := randomOrg()
	sc := domain.Scope{OrgID: org, ProjectID: "prj_p", Subject: "usr_1"}
	errs := make([]error, 8)
	var wg sync.WaitGroup
	for i := range errs {
		wg.Go(func() {
			if i%2 == 0 {
				errs[i] = repo.CreateTenant(ctx, org)
			} else {
				_, errs[i] = repo.Create(ctx, sc, fields("Sensor"))
			}
		})
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("request %d: %v", i, err)
		}
	}
	var n int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM tenants WHERE org_id = $1", org).Scan(&n); err != nil || n != 1 {
		t.Fatalf("tenant rows = %d, %v; want 1", n, err)
	}
	if ok, err := repo.HasTenant(ctx, org); err != nil || !ok {
		t.Fatalf("HasTenant = %v, %v", ok, err)
	}
}

func TestMigrateTwiceIsANoOp(t *testing.T) {
	_, pool := open(t)
	if err := postgres.Migrate(context.Background(), pool, migrations.FS); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
}
