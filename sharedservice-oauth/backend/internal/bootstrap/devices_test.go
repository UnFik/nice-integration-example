package bootstrap

import (
	"net/http"
	"strings"
	"testing"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

func TestDeviceLifecycle(t *testing.T) {
	e := newTestEnv(t, &fakeChecker{allowed: true})
	tok := token(t, orgA, allPerms...)

	r := e.do(http.MethodPost, "/devices", tok, map[string]any{"name": "  Sensor Suhu  ", "serial_number": " SN-1 ", "status": "active", "location": "Gudang A"})
	if r.Status != http.StatusCreated {
		t.Fatalf("create = %d %s", r.Status, r.Body)
	}
	created := r.JSON(t)
	id, _ := created["id"].(string)
	if !strings.HasPrefix(id, "dev_") || created["name"] != "Sensor Suhu" || created["serial_number"] != "SN-1" || created["status"] != "active" || created["location"] != "Gudang A" || created["created_by"] != "usr_test" {
		t.Fatalf("created = %v", created)
	}
	if loc := r.Header.Get("Location"); loc != "/devices/"+id {
		t.Fatalf("Location = %q", loc)
	}

	if r = e.do(http.MethodGet, "/devices/"+id, tok, nil); r.Status != http.StatusOK || r.JSON(t)["name"] != "Sensor Suhu" {
		t.Fatalf("get = %d %s", r.Status, r.Body)
	}
	r = e.do(http.MethodPut, "/devices/"+id, tok, map[string]any{"name": "Sensor Suhu Gudang", "serial_number": "SN-2", "status": "maintenance"})
	if r.Status != http.StatusOK || r.JSON(t)["name"] != "Sensor Suhu Gudang" || r.JSON(t)["serial_number"] != "SN-2" || r.JSON(t)["status"] != "maintenance" || r.JSON(t)["location"] != "" {
		t.Fatalf("update = %d %s", r.Status, r.Body)
	}
	r = e.do(http.MethodGet, "/devices", tok, nil)
	if r.Status != http.StatusOK || !strings.Contains(r.Body, id) {
		t.Fatalf("list = %d %s", r.Status, r.Body)
	}
	if r = e.do(http.MethodDelete, "/devices/"+id, tok, nil); r.Status != http.StatusNoContent {
		t.Fatalf("delete = %d %s", r.Status, r.Body)
	}
	if r = e.do(http.MethodGet, "/devices/"+id, tok, nil); r.Status != http.StatusNotFound {
		t.Fatalf("get after delete = %d", r.Status)
	}
}

func TestUnknownDeviceIsNotFound(t *testing.T) {
	e := newTestEnv(t, &fakeChecker{allowed: true})
	tok := token(t, orgA, allPerms...)
	for _, c := range []struct {
		method string
		body   any
	}{
		{http.MethodGet, nil},
		{http.MethodPut, map[string]any{"name": "x", "serial_number": "SN-1", "status": "active"}},
		{http.MethodDelete, nil},
	} {
		r := e.do(c.method, "/devices/dev_missing", tok, c.body)
		if r.Status != http.StatusNotFound || strings.TrimSpace(r.Body) != `{"error":"not_found"}` {
			t.Errorf("%s unknown device = %d %s, want 404 {\"error\":\"not_found\"}", c.method, r.Status, r.Body)
		}
	}
}

func TestCreateRefusesInvalidInput(t *testing.T) {
	e := newTestEnv(t, nil)
	tok := token(t, orgA, allPerms...)
	for _, c := range []struct {
		name, body, field string
	}{
		{"empty name", `{"name":"","serial_number": "SN-1", "status": "active"}`, "name"},
		{"blank name", `{"name":"   ","serial_number": "SN-1", "status": "active"}`, "name"},
		{"name of 121 characters", `{"name":"` + strings.Repeat("é", 121) + `","serial_number": "SN-1", "status": "active"}`, "name"},
		{"missing name", `{"serial_number": "SN-1", "status": "active"}`, "name"},
		{"name not a string", `{"name":5,"serial_number": "SN-1", "status": "active"}`, "name"},
		{"missing serial number", `{"name":"Sensor","status":"active"}`, "serial_number"},
		{"blank serial number", `{"name":"Sensor","serial_number":" ","status":"active"}`, "serial_number"},
		{"serial number not a string", `{"name":"Sensor","serial_number":7,"status":"active"}`, "serial_number"},
		{"missing status", `{"name":"Sensor","serial_number":"SN-1"}`, "status"},
		{"unknown status", `{"name":"Sensor","serial_number":"SN-1","status":"broken"}`, "status"},
		{"location of 121 characters", `{"name":"Sensor","serial_number":"SN-1","status":"active","location":"` + strings.Repeat("é", 121) + `"}`, "location"},
		{"unknown field", `{"name":"Sensor","serial_number": "SN-1", "status": "active","org":"org_x"}`, "body"},
		{"not JSON", `name=Sensor`, "body"},
		{"two objects", `{"name":"Sensor","serial_number": "SN-1", "status": "active"}{}`, "body"},
	} {
		t.Run(c.name, func(t *testing.T) {
			wantError(t, e.do(http.MethodPost, "/devices", tok, c.body), http.StatusBadRequest, "invalid_argument", c.field)
		})
	}
	if devices := e.list(orgA); len(devices) != 0 {
		t.Fatalf("an invalid request created %v", devices)
	}
}

func TestNameLimitCountsCharactersNotBytes(t *testing.T) {
	e := newTestEnv(t, nil)
	r := e.do(http.MethodPost, "/devices", token(t, orgA, domain.PermCreate), `{"name":"`+strings.Repeat("é", 120)+`","serial_number": "SN-1", "status": "active"}`)
	if r.Status != http.StatusCreated {
		t.Fatalf("a name of 120 characters = %d %s, want 201", r.Status, r.Body)
	}
}

func TestUpdateRefusesInvalidInputAndKeepsTheDevice(t *testing.T) {
	e := newTestEnv(t, nil)
	it := e.create(orgA, "Sensor")
	r := e.do(http.MethodPut, "/devices/"+it.ID, token(t, orgA, domain.PermEdit), `{"name":"Sensor","serial_number":"SN-9","status":"broken"}`)
	wantError(t, r, http.StatusBadRequest, "invalid_argument", "status")
	if got, _ := e.get(orgA, it.ID); got.SerialNumber != it.SerialNumber || got.Status != it.Status {
		t.Fatalf("device changed to %+v", got)
	}
}

func TestDeviceCreatedByAnAgentRecordsTheActor(t *testing.T) {
	e := newTestEnv(t, nil)
	c := claims(orgA, domain.PermCreate, domain.PermRead)
	c["act"] = map[string]any{"sub": "agen-backend-client"}
	tok := sign(t, c)
	r := e.do(http.MethodPost, "/devices", tok, map[string]any{"name": "Sensor", "serial_number": "SN-1", "status": "active"})
	if r.Status != http.StatusCreated {
		t.Fatalf("create = %d %s", r.Status, r.Body)
	}
	created := r.JSON(t)
	if created["created_by"] != "usr_test" || created["acted_by"] != "agen-backend-client" {
		t.Fatalf("created = %v, want created_by the user and acted_by the agent's client", created)
	}
	r = e.do(http.MethodGet, "/devices/"+created["id"].(string), tok, nil)
	if r.JSON(t)["acted_by"] != "agen-backend-client" {
		t.Fatalf("get = %s, want acted_by kept", r.Body)
	}
}

func TestDeviceCreatedByTheUserHasNoActor(t *testing.T) {
	e := newTestEnv(t, nil)
	r := e.do(http.MethodPost, "/devices", token(t, orgA, domain.PermCreate), map[string]any{"name": "Sensor", "serial_number": "SN-1", "status": "active"})
	if r.Status != http.StatusCreated {
		t.Fatalf("create = %d %s", r.Status, r.Body)
	}
	if _, ok := r.JSON(t)["acted_by"]; ok {
		t.Fatalf("created = %s, want no acted_by without act", r.Body)
	}
}
