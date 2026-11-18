package deviceusecase_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	devicememory "github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/memory/device"
	devicedto "github.com/nice-platform/example/sharedservice-oauth/backend/internal/app/device/dto"
	deviceusecase "github.com/nice-platform/example/sharedservice-oauth/backend/internal/app/device/usecase"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

var (
	scopeAP = domain.Scope{OrgID: "org_a", ProjectID: "prj_p", Subject: "usr_1"}
	scopeAQ = domain.Scope{OrgID: "org_a", ProjectID: "prj_q", Subject: "usr_1"}
	scopeBR = domain.Scope{OrgID: "org_b", ProjectID: "prj_r", Subject: "usr_2"}
)

func input(name string, serial int) devicedto.DeviceInput {
	sn, status := fmt.Sprintf("SN-%d", serial), domain.StatusActive
	return devicedto.DeviceInput{Name: &name, SerialNumber: &sn, Status: &status}
}

func TestDevicesStayInTheirOrganizationAndProject(t *testing.T) {
	ctx := context.Background()
	svc := &deviceusecase.Service{Repo: devicememory.NewRepository()}
	created := map[domain.Scope]domain.Device{}
	for _, sc := range []domain.Scope{scopeAP, scopeAQ, scopeBR} {
		it, err := svc.CreateDevice(ctx, sc, input("Sensor "+sc.ProjectID, 1))
		if err != nil {
			t.Fatal(err)
		}
		if it.OrgID != sc.OrgID || it.ProjectID != sc.ProjectID || it.CreatedBy != sc.Subject {
			t.Fatalf("created %+v in scope %+v", it, sc)
		}
		created[sc] = it
	}
	for sc, own := range created {
		devices, err := svc.ListDevices(ctx, sc)
		if err != nil {
			t.Fatal(err)
		}
		if len(devices) != 1 || devices[0].ID != own.ID {
			t.Errorf("list of %+v = %+v, want only %s", sc, devices, own.ID)
		}
		for other, it := range created {
			if other == sc {
				continue
			}
			if _, err := svc.GetDevice(ctx, sc, it.ID); !errors.Is(err, domain.ErrDeviceNotFound) {
				t.Errorf("%+v reading the device of %+v: err = %v, want not found", sc, other, err)
			}
			if _, err := svc.UpdateDevice(ctx, sc, it.ID, input("Diambil", 9)); !errors.Is(err, domain.ErrDeviceNotFound) {
				t.Errorf("%+v editing the device of %+v: err = %v, want not found", sc, other, err)
			}
			if err := svc.DeleteDevice(ctx, sc, it.ID); !errors.Is(err, domain.ErrDeviceNotFound) {
				t.Errorf("%+v deleting the device of %+v: err = %v, want not found", sc, other, err)
			}
		}
	}
	for sc, it := range created {
		if got, err := svc.GetDevice(ctx, sc, it.ID); err != nil || got.Name != it.Name {
			t.Errorf("device of %+v after the other scopes tried = %+v, %v", sc, got, err)
		}
	}
}

func TestEveryUseCaseRefusesAScopeWithoutProject(t *testing.T) {
	ctx := context.Background()
	repo := devicememory.NewRepository()
	svc := &deviceusecase.Service{Repo: repo}
	it, err := svc.CreateDevice(ctx, scopeAP, input("Sensor", 1))
	if err != nil {
		t.Fatal(err)
	}
	noProject := domain.Scope{OrgID: scopeAP.OrgID, Subject: scopeAP.Subject}
	_, errList := svc.ListDevices(ctx, noProject)
	_, errGet := svc.GetDevice(ctx, noProject, it.ID)
	_, errCreate := svc.CreateDevice(ctx, noProject, input("Baru", 1))
	_, errUpdate := svc.UpdateDevice(ctx, noProject, it.ID, input("Baru", 1))
	errDelete := svc.DeleteDevice(ctx, noProject, it.ID)
	for name, err := range map[string]error{"list": errList, "get": errGet, "create": errCreate, "update": errUpdate, "delete": errDelete} {
		if !errors.Is(err, domain.ErrProjectRequired) {
			t.Errorf("%s without project: err = %v, want %v", name, err, domain.ErrProjectRequired)
		}
	}
	devices, _ := svc.ListDevices(ctx, scopeAP)
	if len(devices) != 1 || devices[0].Name != "Sensor" {
		t.Fatalf("devices after the refusals = %+v", devices)
	}
}

func TestCreateRecordsTheActorOfTheScope(t *testing.T) {
	sc := scopeAP
	sc.Actor = "cli_agent"
	it, err := (&deviceusecase.Service{Repo: devicememory.NewRepository()}).CreateDevice(context.Background(), sc, input("Sensor", 1))
	if err != nil {
		t.Fatal(err)
	}
	if it.CreatedBy != "usr_1" || it.ActedBy != "cli_agent" {
		t.Fatalf("created_by %q acted_by %q, want the user and the actor of the scope", it.CreatedBy, it.ActedBy)
	}
}

func TestInvalidInputCreatesNothing(t *testing.T) {
	ctx := context.Background()
	svc := &deviceusecase.Service{Repo: devicememory.NewRepository()}
	var bad *domain.InvalidFieldError
	if _, err := svc.CreateDevice(ctx, scopeAP, input("", 1)); !errors.As(err, &bad) || bad.Field != "name" {
		t.Fatalf("err = %v, want invalid name", err)
	}
	if devices, _ := svc.ListDevices(ctx, scopeAP); len(devices) != 0 {
		t.Fatalf("devices = %+v", devices)
	}
}
