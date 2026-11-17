package itemusecase_test

import (
	"context"
	"errors"
	"testing"

	itemmemory "github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/memory/item"
	itemdto "github.com/nice-platform/example/sharedservice-oauth/backend/internal/app/item/dto"
	itemusecase "github.com/nice-platform/example/sharedservice-oauth/backend/internal/app/item/usecase"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

var (
	scopeAP = domain.Scope{OrgID: "org_a", ProjectID: "prj_p", Subject: "usr_1"}
	scopeAQ = domain.Scope{OrgID: "org_a", ProjectID: "prj_q", Subject: "usr_1"}
	scopeBR = domain.Scope{OrgID: "org_b", ProjectID: "prj_r", Subject: "usr_2"}
)

func input(name string, quantity int64) itemdto.ItemInput {
	return itemdto.ItemInput{Name: &name, Quantity: &quantity}
}

func TestItemsStayInTheirOrganizationAndProject(t *testing.T) {
	ctx := context.Background()
	svc := &itemusecase.Service{Repo: itemmemory.NewRepository()}
	created := map[domain.Scope]domain.Item{}
	for _, sc := range []domain.Scope{scopeAP, scopeAQ, scopeBR} {
		it, err := svc.CreateItem(ctx, sc, input("Kabel "+sc.ProjectID, 1))
		if err != nil {
			t.Fatal(err)
		}
		if it.OrgID != sc.OrgID || it.ProjectID != sc.ProjectID || it.CreatedBy != sc.Subject {
			t.Fatalf("created %+v in scope %+v", it, sc)
		}
		created[sc] = it
	}
	for sc, own := range created {
		items, err := svc.ListItems(ctx, sc)
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 1 || items[0].ID != own.ID {
			t.Errorf("list of %+v = %+v, want only %s", sc, items, own.ID)
		}
		for other, it := range created {
			if other == sc {
				continue
			}
			if _, err := svc.GetItem(ctx, sc, it.ID); !errors.Is(err, domain.ErrItemNotFound) {
				t.Errorf("%+v reading the item of %+v: err = %v, want not found", sc, other, err)
			}
			if _, err := svc.UpdateItem(ctx, sc, it.ID, input("Diambil", 9)); !errors.Is(err, domain.ErrItemNotFound) {
				t.Errorf("%+v editing the item of %+v: err = %v, want not found", sc, other, err)
			}
			if err := svc.DeleteItem(ctx, sc, it.ID); !errors.Is(err, domain.ErrItemNotFound) {
				t.Errorf("%+v deleting the item of %+v: err = %v, want not found", sc, other, err)
			}
		}
	}
	for sc, it := range created {
		if got, err := svc.GetItem(ctx, sc, it.ID); err != nil || got.Name != it.Name {
			t.Errorf("item of %+v after the other scopes tried = %+v, %v", sc, got, err)
		}
	}
}

func TestEveryUseCaseRefusesAScopeWithoutProject(t *testing.T) {
	ctx := context.Background()
	repo := itemmemory.NewRepository()
	svc := &itemusecase.Service{Repo: repo}
	it, err := svc.CreateItem(ctx, scopeAP, input("Kabel", 1))
	if err != nil {
		t.Fatal(err)
	}
	noProject := domain.Scope{OrgID: scopeAP.OrgID, Subject: scopeAP.Subject}
	_, errList := svc.ListItems(ctx, noProject)
	_, errGet := svc.GetItem(ctx, noProject, it.ID)
	_, errCreate := svc.CreateItem(ctx, noProject, input("Baru", 1))
	_, errUpdate := svc.UpdateItem(ctx, noProject, it.ID, input("Baru", 1))
	errDelete := svc.DeleteItem(ctx, noProject, it.ID)
	for name, err := range map[string]error{"list": errList, "get": errGet, "create": errCreate, "update": errUpdate, "delete": errDelete} {
		if !errors.Is(err, domain.ErrProjectRequired) {
			t.Errorf("%s without project: err = %v, want %v", name, err, domain.ErrProjectRequired)
		}
	}
	items, _ := svc.ListItems(ctx, scopeAP)
	if len(items) != 1 || items[0].Name != "Kabel" {
		t.Fatalf("items after the refusals = %+v", items)
	}
}

func TestCreateRecordsTheActorOfTheScope(t *testing.T) {
	sc := scopeAP
	sc.Actor = "cli_agent"
	it, err := (&itemusecase.Service{Repo: itemmemory.NewRepository()}).CreateItem(context.Background(), sc, input("Kabel", 1))
	if err != nil {
		t.Fatal(err)
	}
	if it.CreatedBy != "usr_1" || it.ActedBy != "cli_agent" {
		t.Fatalf("created_by %q acted_by %q, want the user and the actor of the scope", it.CreatedBy, it.ActedBy)
	}
}

func TestInvalidInputCreatesNothing(t *testing.T) {
	ctx := context.Background()
	svc := &itemusecase.Service{Repo: itemmemory.NewRepository()}
	var bad *domain.InvalidFieldError
	if _, err := svc.CreateItem(ctx, scopeAP, input("", 1)); !errors.As(err, &bad) || bad.Field != "name" {
		t.Fatalf("err = %v, want invalid name", err)
	}
	if items, _ := svc.ListItems(ctx, scopeAP); len(items) != 0 {
		t.Fatalf("items = %+v", items)
	}
}
