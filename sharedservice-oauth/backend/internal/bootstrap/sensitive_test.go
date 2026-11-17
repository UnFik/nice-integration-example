package bootstrap

import (
	"errors"
	"net/http"
	"slices"
	"testing"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

func TestDeleteRefusedByNICEKeepsTheItem(t *testing.T) {
	checker := &fakeChecker{allowed: false}
	e := newTestEnv(t, checker)
	it := e.create(orgA, "Kabel")
	r := e.do(http.MethodDelete, "/items/"+it.ID, token(t, orgA, allPerms...), nil)
	wantError(t, r, http.StatusForbidden, "insufficient_scope", "access_denied")
	if _, ok := e.get(orgA, it.ID); !ok {
		t.Fatal("the item was deleted although NICE refused")
	}
	if want := []string{"usr_test " + orgA + " " + domain.PermDelete}; !slices.Equal(checker.Calls(), want) {
		t.Fatalf("checks = %v, want %v", checker.Calls(), want)
	}
}

func TestDeleteWithoutCheckerAnswers503AndKeepsTheItem(t *testing.T) {
	e := newTestEnv(t, nil)
	it := e.create(orgA, "Kabel")
	r := e.do(http.MethodDelete, "/items/"+it.ID, token(t, orgA, allPerms...), nil)
	wantError(t, r, http.StatusServiceUnavailable, "temporarily_unavailable", "access_check_unavailable")
	if _, ok := e.get(orgA, it.ID); !ok {
		t.Fatal("the item was deleted without an online check")
	}
}

func TestDeleteWhenTheCheckFailsAnswers503AndKeepsTheItem(t *testing.T) {
	e := newTestEnv(t, &fakeChecker{err: errors.New("NICE unreachable")})
	it := e.create(orgA, "Kabel")
	r := e.do(http.MethodDelete, "/items/"+it.ID, token(t, orgA, allPerms...), nil)
	wantError(t, r, http.StatusServiceUnavailable, "temporarily_unavailable", "access_check_unavailable")
	if _, ok := e.get(orgA, it.ID); !ok {
		t.Fatal("the item was deleted although the check failed")
	}
}

func TestDeleteChecksThePermissionBeforeAskingNICE(t *testing.T) {
	checker := &fakeChecker{allowed: true}
	e := newTestEnv(t, checker)
	it := e.create(orgA, "Kabel")
	r := e.do(http.MethodDelete, "/items/"+it.ID, token(t, orgA, domain.PermRead, domain.PermEdit), nil)
	wantError(t, r, http.StatusForbidden, "insufficient_scope", "permission_denied")
	if len(checker.Calls()) != 0 {
		t.Fatalf("NICE was asked although the token lacks %s", domain.PermDelete)
	}
}
