package main

import (
	"net/http"
	"strings"
	"testing"
)

func TestItemLifecycle(t *testing.T) {
	e := newTestEnv(t, &fakeChecker{allowed: true})
	tok := token(t, orgA, allPerms...)

	r := e.do(http.MethodPost, "/items", tok, map[string]any{"name": "  Kabel UTP  ", "quantity": 3})
	if r.Status != http.StatusCreated {
		t.Fatalf("create = %d %s", r.Status, r.Body)
	}
	created := r.JSON(t)
	id, _ := created["id"].(string)
	if !strings.HasPrefix(id, "itm_") || created["name"] != "Kabel UTP" || created["quantity"] != float64(3) || created["created_by"] != "usr_test" {
		t.Fatalf("created = %v", created)
	}
	if loc := r.Header.Get("Location"); loc != "/items/"+id {
		t.Fatalf("Location = %q", loc)
	}

	if r = e.do(http.MethodGet, "/items/"+id, tok, nil); r.Status != http.StatusOK || r.JSON(t)["name"] != "Kabel UTP" {
		t.Fatalf("get = %d %s", r.Status, r.Body)
	}
	r = e.do(http.MethodPut, "/items/"+id, tok, map[string]any{"name": "Kabel UTP Cat6", "quantity": 0})
	if r.Status != http.StatusOK || r.JSON(t)["name"] != "Kabel UTP Cat6" || r.JSON(t)["quantity"] != float64(0) {
		t.Fatalf("update = %d %s", r.Status, r.Body)
	}
	r = e.do(http.MethodGet, "/items", tok, nil)
	if r.Status != http.StatusOK || !strings.Contains(r.Body, id) {
		t.Fatalf("list = %d %s", r.Status, r.Body)
	}
	if r = e.do(http.MethodDelete, "/items/"+id, tok, nil); r.Status != http.StatusNoContent {
		t.Fatalf("delete = %d %s", r.Status, r.Body)
	}
	if r = e.do(http.MethodGet, "/items/"+id, tok, nil); r.Status != http.StatusNotFound {
		t.Fatalf("get after delete = %d", r.Status)
	}
}

func TestUnknownItemIsNotFound(t *testing.T) {
	e := newTestEnv(t, &fakeChecker{allowed: true})
	tok := token(t, orgA, allPerms...)
	for _, c := range []struct {
		method string
		body   any
	}{
		{http.MethodGet, nil},
		{http.MethodPut, map[string]any{"name": "x", "quantity": 1}},
		{http.MethodDelete, nil},
	} {
		r := e.do(c.method, "/items/itm_missing", tok, c.body)
		if r.Status != http.StatusNotFound || strings.TrimSpace(r.Body) != `{"error":"not_found"}` {
			t.Errorf("%s unknown item = %d %s, want 404 {\"error\":\"not_found\"}", c.method, r.Status, r.Body)
		}
	}
}

func TestCreateRefusesInvalidInput(t *testing.T) {
	e := newTestEnv(t, nil)
	tok := token(t, orgA, allPerms...)
	for _, c := range []struct {
		name, body, field string
	}{
		{"empty name", `{"name":"","quantity":1}`, "name"},
		{"blank name", `{"name":"   ","quantity":1}`, "name"},
		{"name of 121 characters", `{"name":"` + strings.Repeat("é", 121) + `","quantity":1}`, "name"},
		{"missing name", `{"quantity":1}`, "name"},
		{"name not a string", `{"name":5,"quantity":1}`, "name"},
		{"negative quantity", `{"name":"Kabel","quantity":-1}`, "quantity"},
		{"missing quantity", `{"name":"Kabel"}`, "quantity"},
		{"fractional quantity", `{"name":"Kabel","quantity":1.5}`, "quantity"},
		{"quantity as a string", `{"name":"Kabel","quantity":"1"}`, "quantity"},
		{"unknown field", `{"name":"Kabel","quantity":1,"org":"org_x"}`, "body"},
		{"not JSON", `name=Kabel`, "body"},
		{"two objects", `{"name":"Kabel","quantity":1}{}`, "body"},
	} {
		t.Run(c.name, func(t *testing.T) {
			wantError(t, e.do(http.MethodPost, "/items", tok, c.body), http.StatusBadRequest, "invalid_argument", c.field)
		})
	}
	if items := e.store.List(orgA); len(items) != 0 {
		t.Fatalf("an invalid request created %v", items)
	}
}

func TestNameLimitCountsCharactersNotBytes(t *testing.T) {
	e := newTestEnv(t, nil)
	r := e.do(http.MethodPost, "/items", token(t, orgA, permCreate), `{"name":"`+strings.Repeat("é", 120)+`","quantity":0}`)
	if r.Status != http.StatusCreated {
		t.Fatalf("a name of 120 characters = %d %s, want 201", r.Status, r.Body)
	}
}

func TestUpdateRefusesInvalidInputAndKeepsTheItem(t *testing.T) {
	e := newTestEnv(t, nil)
	it := e.create(orgA, "Kabel")
	r := e.do(http.MethodPut, "/items/"+it.ID, token(t, orgA, permEdit), `{"name":"Kabel","quantity":-5}`)
	wantError(t, r, http.StatusBadRequest, "invalid_argument", "quantity")
	if got, _ := e.store.Get(orgA, it.ID); got.Quantity != it.Quantity {
		t.Fatalf("quantity changed to %d", got.Quantity)
	}
}
