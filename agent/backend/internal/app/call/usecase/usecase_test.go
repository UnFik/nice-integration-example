package callusecase

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/ABA-Developer/nice-auth-sdk/credentials"

	calldto "github.com/nice-platform/example/agent/backend/internal/app/call/dto"
	"github.com/nice-platform/example/agent/backend/internal/domain"
)

const (
	orgA = "org_aaaa"
	orgB = "org_bbbb"
	prjP = "prj_pppp"
	prjQ = "prj_qqqq"
	prjR = "prj_rrrr"
)

type fakeNICE struct {
	m        calldto.ServiceMap
	ops      map[string]calldto.ServiceOperations
	mapErr   error
	opsErr   error
	calls    []calldto.OperationCall
	callErr  error
	result   calldto.OperationResult
	mapReads int
	// schemas answers OperationSchemas by key; schemaErr fails it; schemaAsks records the keys asked.
	schemas    map[string]calldto.OperationSchema
	schemaErr  error
	schemaAsks [][]string
}

func (f *fakeNICE) OperationSchemas(_ context.Context, _, slug string, keys []string) (calldto.OperationSchemas, error) {
	f.schemaAsks = append(f.schemaAsks, keys)
	if f.schemaErr != nil {
		return calldto.OperationSchemas{}, f.schemaErr
	}
	if _, ok := f.ops[slug]; !ok {
		return calldto.OperationSchemas{}, &domain.APIError{Status: 404, Code: "not_found"}
	}
	out := calldto.OperationSchemas{Service: slug, Schemas: []calldto.OperationSchema{}, Missing: []string{}}
	for _, k := range keys {
		if sc, ok := f.schemas[k]; ok {
			out.Schemas = append(out.Schemas, sc)
		} else {
			out.Missing = append(out.Missing, k)
		}
	}
	return out, nil
}

func (f *fakeNICE) Redact(schema calldto.OperationSchema, status int, body []byte) ([]byte, error) {
	s := credentials.OperationSchema{Key: schema.Key, InputSchema: schema.InputSchema, Responses: schema.Responses}
	return credentials.RedactSecrets(s.ResponseSchema(status), body)
}

func (f *fakeNICE) Services(context.Context, string) (calldto.ServiceMap, error) {
	f.mapReads++
	return f.m, f.mapErr
}

func (f *fakeNICE) Operations(_ context.Context, _, _, slug string) (calldto.ServiceOperations, error) {
	if f.opsErr != nil {
		return calldto.ServiceOperations{}, f.opsErr
	}
	ops, ok := f.ops[slug]
	if !ok {
		return calldto.ServiceOperations{}, &domain.APIError{Status: 404, Code: "not_found"}
	}
	return ops, nil
}

func (f *fakeNICE) CallOperation(_ context.Context, _ string, call calldto.OperationCall) (calldto.OperationResult, error) {
	f.calls = append(f.calls, call)
	return f.result, f.callErr
}

func newFake() *fakeNICE {
	return &fakeNICE{
		m: calldto.ServiceMap{
			Services: []calldto.MapService{{Slug: "nadi", Name: "NADI", Endpoint: "https://nadi.example.com"}},
			Organizations: []calldto.MapOrganization{
				{ID: orgA, Name: "Org A", Projects: []calldto.MapProject{{ID: prjP, Name: "Demo", Services: []string{"nadi"}}}},
				{ID: orgB, Name: "Org B", Projects: []calldto.MapProject{{ID: prjQ, Name: "Lokasi Q", Services: []string{"nadi"}}, {ID: prjR, Services: []string{"other"}}}},
			},
		},
		ops: map[string]calldto.ServiceOperations{"nadi": {Service: "nadi", Operations: []calldto.Operation{
			{Key: "listDevices", Method: "GET", Path: "/devices", RequestFields: []calldto.RequestField{{Location: "query", Name: "status"}}},
			{Key: "createDevice", Method: "POST", Path: "/devices", RequestFields: []calldto.RequestField{{Location: "body", Name: "name"}}},
			{Key: "deleteDevice", Method: "DELETE", Path: "/devices/{id}", Summary: "Delete a device", Sensitive: true,
				RequestFields: []calldto.RequestField{{Location: "path", Name: "id", Required: true}}},
		}}},
		result: calldto.OperationResult{Status: 200, Body: []byte(`{"devices":[]}`)},
		schemas: map[string]calldto.OperationSchema{
			"listDevices":  {Key: "listDevices", InputSchema: []byte(`{"type":"object"}`), Responses: []byte(`{}`)},
			"createDevice": {Key: "createDevice", InputSchema: []byte(`{"type":"object","properties":{"body":{"type":"object","required":["name"]}}}`), Responses: []byte(`{}`)},
			"deleteDevice": {Key: "deleteDevice", InputSchema: []byte(`{"type":"object"}`), Responses: []byte(`{}`)},
		},
	}
}

func newService(f *fakeNICE) *Service {
	return &Service{Discoverer: f, Operator: f, ClientVars: "NICE_CLIENT_ID and NICE_CLIENT_SECRET", Log: slog.New(slog.DiscardHandler)}
}

const userTok = "secret-user-token-7f3a"

var user = calldto.Caller{Token: userTok, Subject: "usr_1", OrgID: orgA, ProjectID: prjP}

func wantFailure(t *testing.T, f *calldto.Failure, code string) {
	t.Helper()
	if f == nil || f.Code != code {
		t.Fatalf("failure = %+v, want %s", f, code)
	}
	raw, _ := json.Marshal(f)
	if strings.Contains(string(raw), userTok) {
		t.Fatalf("failure shows the token: %s", raw)
	}
}

func TestListServicesHidesEndpointsAndMarksTruncation(t *testing.T) {
	f := newFake()
	f.m.Organizations[1].ProjectsTruncated = true
	v, fail := newService(f).ListServices(context.Background(), user)
	if fail != nil {
		t.Fatal(fail)
	}
	raw, _ := json.Marshal(v)
	if strings.Contains(string(raw), "nadi.example.com") {
		t.Fatalf("list_services shows an endpoint: %s", raw)
	}
	if !v.Organizations[1].Incomplete || v.Organizations[0].Incomplete || v.Organizations[0].Projects[0].Services[0] != "nadi" {
		t.Fatalf("view = %s", raw)
	}
}

func TestDiscoveryFailures(t *testing.T) {
	f := newFake()
	f.mapErr = &domain.APIError{Status: 404, Code: "not_found"}
	_, fail := newService(f).ListServices(context.Background(), user)
	wantFailure(t, fail, "discovery_unavailable")

	f = newFake()
	_, fail = newService(f).ListOperations(context.Background(), user, "pengiriman")
	wantFailure(t, fail, "service_not_allowed")

	f.opsErr = &domain.APIError{Status: 403, Code: "delegation_not_allowed"}
	_, fail = newService(f).ListOperations(context.Background(), user, "nadi")
	wantFailure(t, fail, "token_stale_after_upgrade")

	_, fail = newService(f).ListOperations(context.Background(), user, "nadi/../nice")
	wantFailure(t, fail, "invalid_argument")

	f = newFake()
	f.mapErr = &domain.GrantError{Code: "invalid_grant", Description: "subject_token_expiring"}
	_, fail = newService(f).ListServices(context.Background(), user)
	wantFailure(t, fail, "invalid_grant")
}

func TestListOperationsShowsSensitive(t *testing.T) {
	v, fail := newService(newFake()).ListOperations(context.Background(), user, "nadi")
	if fail != nil || len(v.Operations) != 3 || !v.Operations[2].Sensitive || v.Operations[0].Sensitive {
		t.Fatalf("operations = %+v, %v", v, fail)
	}
}

func call(s *Service, in calldto.OperationInput, ap calldto.Approval) (*calldto.CallView, *calldto.ApprovalRequest, *calldto.Failure) {
	return s.CallOperation(context.Background(), user, in, ap)
}

func TestCallInTheContextOfTheToken(t *testing.T) {
	f := newFake()
	out, ask, fail := call(newService(f), calldto.OperationInput{Service: "nadi", Operation: "listDevices", Query: map[string]string{"status": "active"}}, calldto.Approval{})
	if fail != nil || ask != nil || out.Status != 200 {
		t.Fatalf("out %+v ask %+v fail %+v", out, ask, fail)
	}
	c := f.calls[0]
	if c.Org != "" || c.Project != "" || c.Service.Endpoint != "https://nadi.example.com" || c.Query["status"][0] != "active" {
		t.Fatalf("call = %+v", c)
	}
}

func TestCallSendsTheBodyAsJSON(t *testing.T) {
	f := newFake()
	f.result = calldto.OperationResult{Status: 201, Body: []byte(`{"name":"Sensor"}`)}
	out, _, fail := call(newService(f), calldto.OperationInput{Service: "nadi", Operation: "createDevice",
		Body: map[string]any{"name": "Sensor"}}, calldto.Approval{})
	if fail != nil || out.Status != 201 || string(f.calls[0].Body) != `{"name":"Sensor"}` {
		t.Fatalf("out %+v fail %+v body %s", out, fail, f.calls[0].Body)
	}
}

func TestCallInAnotherContextOfTheMap(t *testing.T) {
	f := newFake()
	_, _, fail := call(newService(f), calldto.OperationInput{Service: "nadi", Operation: "listDevices",
		Where: calldto.Where{Org: orgB, Project: prjQ}}, calldto.Approval{})
	if fail != nil || f.calls[0].Org != orgB || f.calls[0].Project != prjQ {
		t.Fatalf("fail %+v calls %+v", fail, f.calls)
	}
}

func TestCallRefusedBeforeAnyCall(t *testing.T) {
	cases := map[string]calldto.OperationInput{
		"bad slug":            {Service: "Nadi", Operation: "listDevices"},
		"no operation":        {Service: "nadi"},
		"unknown operation":   {Service: "nadi", Operation: "rebootDevice"},
		"unknown query":       {Service: "nadi", Operation: "listDevices", Query: map[string]string{"limit": "1"}},
		"path field as query": {Service: "nadi", Operation: "deleteDevice", Query: map[string]string{"id": "1"}},
		"project not in map":  {Service: "nadi", Operation: "listDevices", Where: calldto.Where{Org: orgB, Project: prjR}},
		"org without project": {Service: "nadi", Operation: "listDevices", Where: calldto.Where{Org: orgB}},
		"scope injection":     {Service: "nadi", Operation: "listDevices", Where: calldto.Where{Project: prjP + " nice:aud:nice"}},
	}
	for name, in := range cases {
		f := newFake()
		_, ask, fail := call(newService(f), in, calldto.Approval{})
		if ask != nil || fail == nil || fail.Code != "invalid_argument" || len(f.calls) != 0 {
			t.Errorf("%s: ask %+v fail %+v calls %d, want invalid_argument and no call", name, ask, fail, len(f.calls))
		}
	}
}

func TestCallRefusesAServiceOutsideTheMapAndAnInsecureEndpoint(t *testing.T) {
	f := newFake()
	_, _, fail := call(newService(f), calldto.OperationInput{Service: "pengiriman", Operation: "listShipments"}, calldto.Approval{})
	wantFailure(t, fail, "service_not_allowed")

	f.m.Services[0].Endpoint = "http://nadi:8081"
	_, _, fail = call(newService(f), calldto.OperationInput{Service: "nadi", Operation: "listDevices"}, calldto.Approval{})
	wantFailure(t, fail, "insecure_endpoint")
	if len(f.calls) != 0 {
		t.Fatalf("calls = %d", len(f.calls))
	}
}

func TestCallExplainsTheServiceAnswer(t *testing.T) {
	f := newFake()
	f.result = calldto.OperationResult{Status: 403, Body: []byte(`{"error":"insufficient_scope","error_description":"permission_denied"}`)}
	_, _, fail := call(newService(f), calldto.OperationInput{Service: "nadi", Operation: "listDevices"}, calldto.Approval{})
	wantFailure(t, fail, "insufficient_scope")
	if !strings.Contains(fail.Message, "permission") {
		t.Fatalf("message %q", fail.Message)
	}
	f.result, f.callErr = calldto.OperationResult{}, &domain.InvalidCallError{Reason: `path parameter "id" is missing`}
	_, _, fail = call(newService(f), calldto.OperationInput{Service: "nadi", Operation: "listDevices"}, calldto.Approval{})
	wantFailure(t, fail, "invalid_argument")
	if !strings.Contains(fail.Message, `"id"`) {
		t.Fatalf("message %q does not name the parameter", fail.Message)
	}
}

var del = calldto.OperationInput{Service: "nadi", Operation: "deleteDevice", PathParams: map[string]string{"id": "dev-1"}}

func TestSensitiveOperationAsksThenCallsOnce(t *testing.T) {
	f := newFake()
	s := newService(f)
	_, ask, fail := call(s, del, calldto.Approval{})
	if fail != nil || ask == nil || len(f.calls) != 0 {
		t.Fatalf("first call: ask %+v fail %+v calls %d", ask, fail, len(f.calls))
	}
	for _, want := range []string{"NADI", "Delete a device", "DELETE /devices/dev-1", "Org A", "Demo"} {
		if !strings.Contains(ask.Message, want) {
			t.Errorf("message %q does not name %q", ask.Message, want)
		}
	}
	out, again, fail := call(s, del, calldto.Approval{State: ask.State, Accepted: true})
	if fail != nil || again != nil || out == nil || len(f.calls) != 1 {
		t.Fatalf("approved call: out %+v ask %+v fail %+v calls %d", out, again, fail, len(f.calls))
	}
	// The same state again is not an approval: the user is asked again.
	_, again, _ = call(s, del, calldto.Approval{State: ask.State, Accepted: true})
	if again == nil || len(f.calls) != 1 {
		t.Fatalf("replayed state: ask %+v calls %d", again, len(f.calls))
	}
}

func TestSensitiveOperationDeclined(t *testing.T) {
	f := newFake()
	s := newService(f)
	_, ask, _ := call(s, del, calldto.Approval{})
	_, again, fail := call(s, del, calldto.Approval{State: ask.State})
	wantFailure(t, fail, "not_approved")
	if again != nil || len(f.calls) != 0 {
		t.Fatalf("declined: ask %+v calls %d", again, len(f.calls))
	}
}

func TestApprovalBoundToTheUserAndTheCall(t *testing.T) {
	f := newFake()
	s := newService(f)
	_, ask, _ := call(s, del, calldto.Approval{})
	other := calldto.OperationInput{Service: "nadi", Operation: "deleteDevice", PathParams: map[string]string{"id": "dev-2"}}
	for name, run := range map[string]func() *calldto.ApprovalRequest{
		"other arguments": func() *calldto.ApprovalRequest {
			_, a, _ := call(s, other, calldto.Approval{State: ask.State, Accepted: true})
			return a
		},
		"other user": func() *calldto.ApprovalRequest {
			_, a, _ := s.CallOperation(context.Background(), calldto.Caller{Token: "tok2", Subject: "usr_2", OrgID: orgA}, del,
				calldto.Approval{State: ask.State, Accepted: true})
			return a
		},
		"other service instance": func() *calldto.ApprovalRequest {
			_, a, _ := call(newService(f), del, calldto.Approval{State: ask.State, Accepted: true})
			return a
		},
	} {
		if a := run(); a == nil {
			t.Errorf("%s: no new approval asked", name)
		}
	}
	if len(f.calls) != 0 {
		t.Fatalf("calls = %d, want none", len(f.calls))
	}
}

func TestNonSensitiveOperationAsksNothing(t *testing.T) {
	_, ask, fail := call(newService(newFake()), calldto.OperationInput{Service: "nadi", Operation: "listDevices"}, calldto.Approval{})
	if ask != nil || fail != nil {
		t.Fatalf("ask %+v fail %+v", ask, fail)
	}
}

func TestListOperationsCarriesDescriptionsAndAllowedValues(t *testing.T) {
	f := newFake()
	nadi := f.ops["nadi"]
	nadi.Operations[0].Description = "Devices of the project."
	nadi.Operations[0].RequestFields[0] = calldto.RequestField{Location: "query", Name: "status", Type: "string",
		Description: "State of the device", Enum: []any{"active", "inactive"}}
	f.ops["nadi"] = nadi
	v, fail := newService(f).ListOperations(context.Background(), user, "nadi")
	if fail != nil {
		t.Fatal(fail)
	}
	raw, _ := json.Marshal(v)
	s := string(raw)
	for _, want := range []string{`"description":"Devices of the project."`, `"description":"State of the device"`, `"enum":["active","inactive"]`} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s in %s", want, s)
		}
	}
	if strings.Contains(s, `"format"`) || strings.Contains(s, `"enum_truncated"`) {
		t.Fatalf("what NICE does not send stays out: %s", s)
	}
}

func TestGetOperationSchemasNamesKeysTheUserMayNotCallAsMissing(t *testing.T) {
	f := newFake()
	v, fail := newService(f).GetOperationSchemas(context.Background(), user, "nadi", []string{"createDevice", "ghost"})
	if fail != nil || len(v.Schemas) != 1 || v.Schemas[0].Key != "createDevice" || fmt.Sprint(v.Missing) != "[ghost]" {
		t.Fatalf("view = %+v, %v", v, fail)
	}
	raw, _ := json.Marshal(v)
	if !strings.Contains(string(raw), `"input_schema":{"type":"object","properties":{"body":{"type":"object","required":["name"]}}}`) {
		t.Fatalf("the input schema is passed on as it is: %s", raw)
	}
}

func TestGetOperationSchemasBoundsTheKeys(t *testing.T) {
	f := newFake()
	var eleven []string
	for i := range 11 {
		eleven = append(eleven, fmt.Sprintf("k%d", i))
	}
	for _, keys := range [][]string{nil, eleven, {""}} {
		if _, fail := newService(f).GetOperationSchemas(context.Background(), user, "nadi", keys); fail == nil || fail.Code != "invalid_argument" {
			t.Fatalf("%d keys: %+v", len(keys), fail)
		}
	}
	if len(f.schemaAsks) != 0 {
		t.Fatalf("NICE asked: %v", f.schemaAsks)
	}
}

func TestGetOperationSchemasOfAnOlderNICE(t *testing.T) {
	f := newFake()
	f.schemaErr = &domain.APIError{Status: 404, Code: "not_found"}
	if _, fail := newService(f).GetOperationSchemas(context.Background(), user, "nadi", []string{"listDevices"}); fail == nil || fail.Code != "schemas_unavailable" {
		t.Fatalf("an older NICE: %+v", fail)
	}
	if _, fail := newService(f).GetOperationSchemas(context.Background(), user, "pengiriman", []string{"x"}); fail == nil || fail.Code != "service_not_allowed" {
		t.Fatalf("a service out of reach: %+v", fail)
	}
}

const secretList = `{"200":{"schema":{"type":"object","properties":{"devices":{"type":"array","items":{"type":"object",
	"properties":{"id":{"type":"string"},"acted_by":{"type":"string","x-secret":true}}}}}}}}`

func TestCallRemovesSecretPropertiesBeforeTheModelSeesThem(t *testing.T) {
	f := newFake()
	sc := f.schemas["listDevices"]
	sc.Responses = []byte(secretList)
	f.schemas["listDevices"] = sc
	f.result = calldto.OperationResult{Status: 200, Body: []byte(`{"devices":[{"id":"d1","acted_by":"cli_x","name":"Pump"}],"total":1}`)}
	out, _, fail := call(newService(f), calldto.OperationInput{Service: "nadi", Operation: "listDevices"}, calldto.Approval{})
	if fail != nil {
		t.Fatal(fail)
	}
	raw, _ := json.Marshal(out.Body)
	if strings.Contains(string(raw), "acted_by") || strings.Contains(string(raw), "cli_x") || !strings.Contains(string(raw), `"name":"Pump"`) {
		t.Fatalf("body = %s", raw)
	}
	if len(f.schemaAsks) != 1 || fmt.Sprint(f.schemaAsks[0]) != "[listDevices]" {
		t.Fatalf("schema asks = %v", f.schemaAsks)
	}
}

func TestCallIsNotSentWhenTheSchemaCannotBeRead(t *testing.T) {
	f := newFake()
	f.schemaErr = domain.ErrTemporary
	_, _, fail := call(newService(f), calldto.OperationInput{Service: "nadi", Operation: "listDevices"}, calldto.Approval{})
	if fail == nil || len(f.calls) != 0 {
		t.Fatalf("fail %+v, calls %d", fail, len(f.calls))
	}
}

func TestCallGoesOnAgainstANICEWithoutSchemas(t *testing.T) {
	f := newFake()
	f.schemaErr = &domain.APIError{Status: 404, Code: "not_found"}
	f.result = calldto.OperationResult{Status: 200, Body: []byte(`{"devices":[{"acted_by":"cli_x"}]}`)}
	out, _, fail := call(newService(f), calldto.OperationInput{Service: "nadi", Operation: "listDevices"}, calldto.Approval{})
	raw, _ := json.Marshal(out)
	if fail != nil || len(f.calls) != 1 || !strings.Contains(string(raw), "cli_x") {
		t.Fatalf("out %s, fail %+v", raw, fail)
	}
}

func TestCallOfAKeyMissingFromTheSchemasIsRefused(t *testing.T) {
	f := newFake()
	delete(f.schemas, "listDevices")
	_, _, fail := call(newService(f), calldto.OperationInput{Service: "nadi", Operation: "listDevices"}, calldto.Approval{})
	if fail == nil || fail.Code != "invalid_argument" || len(f.calls) != 0 {
		t.Fatalf("fail %+v, calls %d", fail, len(f.calls))
	}
}

func TestSensitiveCallStillAsksAfterTheSchemaIsRead(t *testing.T) {
	f := newFake()
	_, ask, fail := call(newService(f), calldto.OperationInput{Service: "nadi", Operation: "deleteDevice", PathParams: map[string]string{"id": "d1"}}, calldto.Approval{})
	if fail != nil || ask == nil || len(f.calls) != 0 || len(f.schemaAsks) != 1 {
		t.Fatalf("ask %+v, fail %+v, calls %d, schema asks %d", ask, fail, len(f.calls), len(f.schemaAsks))
	}
}

func TestErrorAnswerIsCleanedBeforeItsCodeIsRead(t *testing.T) {
	f := newFake()
	f.result = calldto.OperationResult{Status: 503, Body: []byte("<html>down</html>")}
	_, _, fail := call(newService(f), calldto.OperationInput{Service: "nadi", Operation: "listDevices"}, calldto.Approval{})
	if fail == nil || fail.Code != "http_503" || strings.Contains(fail.Message+fail.Description, "html") {
		t.Fatalf("fail %+v", fail)
	}
}
