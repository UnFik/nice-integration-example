package niceexchange

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ABA-Developer/nice-auth-sdk/credentials"

	calldto "github.com/nice-platform/example/agent/backend/internal/app/call/dto"
	"github.com/nice-platform/example/agent/backend/internal/domain"
)

// jwt is an unsigned token: the SDK only reads the subject and the expiry of a subject token, NICE verifies it.
func jwt(claims map[string]any) string {
	enc := func(v any) string { b, _ := json.Marshal(v); return base64.RawURLEncoding.EncodeToString(b) }
	return enc(map[string]string{"alg": "none"}) + "." + enc(claims) + ".sig"
}

func userToken() string {
	return jwt(map[string]any{"sub": "usr_1", "exp": time.Now().Add(2 * time.Hour).Unix(), "aud": "agen"})
}

// fakeNICE answers the token exchange, the service map in pages, and the operations of one service.
type fakeNICE struct {
	srv *httptest.Server

	mu        sync.Mutex
	scopes    []string
	mapStatus int
	pages     []string
	staleOnce bool
	svcCalls  []string
	// opsTag is the ETag of the operations of nadi; opsGone answers them 404; opsAsked records If-None-Match.
	opsTag   string
	opsGone  bool
	opsAsked []string
}

func newFakeNICE(t *testing.T) *fakeNICE {
	f := &fakeNICE{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /oauth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		f.mu.Lock()
		f.scopes = append(f.scopes, r.PostForm.Get("scope"))
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": jwt(map[string]any{"sub": "usr_1", "exp": time.Now().Add(time.Hour).Unix()}),
			"issued_token_type": "urn:ietf:params:oauth:token-type:access_token", "token_type": "Bearer", "expires_in": 3600})
	})
	mux.HandleFunc("GET /api/v1/me/services", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		status, pages := f.mapStatus, f.pages
		f.mu.Unlock()
		if status != 0 {
			w.WriteHeader(status)
			_, _ = fmt.Fprint(w, `{"error_code":"not_found","message":"no such route"}`)
			return
		}
		i := 0
		if tok := r.URL.Query().Get("page_token"); tok != "" {
			_, _ = fmt.Sscan(tok, &i)
		}
		next := ""
		if i+1 < len(pages) {
			next = fmt.Sprint(i + 1)
		}
		_, _ = fmt.Fprintf(w, `%s, "page": {"next_page_token": %q}}`, strings.TrimSuffix(pages[i], "}"), next)
	})
	mux.HandleFunc("GET /api/v1/me/services/{slug}/operations", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		tag, gone := f.opsTag, f.opsGone
		if r.PathValue("slug") == "nadi" {
			f.opsAsked = append(f.opsAsked, r.Header.Get("If-None-Match"))
		}
		f.mu.Unlock()
		if r.PathValue("slug") != "nadi" || gone {
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprint(w, `{"error_code":"not_found","message":"service not found"}`)
			return
		}
		if tag != "" {
			if r.Header.Get("If-None-Match") == tag {
				w.Header().Set("ETag", tag)
				w.WriteHeader(http.StatusNotModified)
				return
			}
			w.Header().Set("ETag", tag)
		}
		_, _ = fmt.Fprint(w, `{"service":"nadi","catalog_version":"7","operations":[
			{"key":"getDevice","method":"GET","path":"/devices/{id}","permission":"nadi.devices.read",
			 "description":"One device of the project.",
			 "headers":[{"name":"X-Version","value":"1"}],
			 "request_fields":[{"location":"path","name":"id","required":true,"type":"string","description":"Device ID"},
			   {"location":"query","name":"status","type":"string","enum":["active","inactive"],"enum_truncated":true}]},
			{"key":"deleteDevice","method":"DELETE","path":"/devices/{id}","sensitive":true,
			 "request_fields":[{"location":"path","name":"id","required":true}]}]}`)
	})
	mux.HandleFunc("/svc/", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.svcCalls = append(f.svcCalls, r.Method+" "+r.URL.EscapedPath()+" "+r.Header.Get("X-Version"))
		stale := f.staleOnce
		f.staleOnce = false
		f.mu.Unlock()
		if stale {
			w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token", error_description="token_stale"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = fmt.Fprint(w, `{"id":"dev 01"}`)
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	f.pages = []string{
		`{"services":[{"slug":"nadi","name":"NADI","endpoint":"` + f.srv.URL + `","base_path":"/svc"}],
		  "organizations":[{"id":"org_a","name":"A","projects":[{"id":"prj_p","name":"P","services":["nadi"]}]}]}`,
		`{"services":[{"slug":"nadi","name":"NADI","endpoint":"` + f.srv.URL + `","base_path":"/svc"}],
		  "organizations":[{"id":"org_b","name":"B","projects":[],"projects_truncated":true}]}`,
	}
	return f
}

func newClient(t *testing.T, f *fakeNICE) *Client {
	c, err := New(credentials.ExchangeConfig{Issuer: f.srv.URL, ClientID: "agen-backend", ClientSecret: "secret", HTTP: f.srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestServicesReadsEveryPageOnce(t *testing.T) {
	f := newFakeNICE(t)
	m, err := newClient(t, f).Services(context.Background(), userToken())
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Services) != 1 || m.Services[0].Slug != "nadi" || m.Services[0].BasePath != "/svc" {
		t.Fatalf("services = %+v", m.Services)
	}
	if len(m.Organizations) != 2 || m.Organizations[0].Projects[0].Services[0] != "nadi" || !m.Organizations[1].ProjectsTruncated {
		t.Fatalf("organizations = %+v", m.Organizations)
	}
	if len(f.scopes) != 1 || f.scopes[0] != "nice:aud:nice" {
		t.Fatalf("exchanges = %v, want one for the NICE API", f.scopes)
	}
}

func TestServicesOfAnOldNICEIsAnAPIError(t *testing.T) {
	f := newFakeNICE(t)
	f.mapStatus = http.StatusNotFound
	_, err := newClient(t, f).Services(context.Background(), userToken())
	var api *domain.APIError
	if !errors.As(err, &api) || api.Status != 404 {
		t.Fatalf("err = %v, want a domain.APIError 404", err)
	}
}

func TestOperationsOfAServiceTheUserMayNotCall(t *testing.T) {
	f := newFakeNICE(t)
	c := newClient(t, f)
	ops, err := c.Operations(context.Background(), userToken(), "usr_1", "nadi")
	if err != nil || len(ops.Operations) != 2 || !ops.Operations[1].Sensitive || ops.Operations[0].RequestFields[0].Name != "id" {
		t.Fatalf("operations = %+v, %v", ops, err)
	}
	_, err = c.Operations(context.Background(), userToken(), "usr_1", "pengiriman")
	var api *domain.APIError
	if !errors.As(err, &api) || api.Status != 404 || api.Code != "not_found" {
		t.Fatalf("err = %v, want a domain.APIError 404 not_found", err)
	}
}

func call(f *fakeNICE, c *Client, id string) (calldto.OperationResult, error) {
	ops, _ := c.Operations(context.Background(), userToken(), "usr_1", "nadi")
	return c.CallOperation(context.Background(), userToken(), calldto.OperationCall{
		Service:   calldto.MapService{Slug: "nadi", Endpoint: f.srv.URL, BasePath: "/svc"},
		Operation: ops.Operations[0], Org: "org_a", Project: "prj_p", PathParams: map[string]string{"id": id},
	})
}

func TestCallOperationSendsTheOperationHeadersAndEscapesThePath(t *testing.T) {
	f := newFakeNICE(t)
	res, err := call(f, newClient(t, f), "dev 01")
	if err != nil || res.Status != 200 || string(res.Body) != `{"id":"dev 01"}` {
		t.Fatalf("result = %d %s, %v", res.Status, res.Body, err)
	}
	if len(f.svcCalls) != 1 || f.svcCalls[0] != "GET /svc/devices/dev%2001 1" {
		t.Fatalf("service calls = %v", f.svcCalls)
	}
}

func TestCallOperationRefusesAPathThatMovesBeforeAnyExchange(t *testing.T) {
	f := newFakeNICE(t)
	c := newClient(t, f)
	ops, _ := c.Operations(context.Background(), userToken(), "usr_1", "nadi")
	before := len(f.scopes)
	_, err := c.CallOperation(context.Background(), userToken(), calldto.OperationCall{
		Service: calldto.MapService{Slug: "nadi", Endpoint: f.srv.URL}, Operation: ops.Operations[0],
		PathParams: map[string]string{"id": ".."},
	})
	var bad *domain.InvalidCallError
	if !errors.As(err, &bad) || strings.Contains(bad.Reason, "niceauth") {
		t.Fatalf("err = %v, want a domain.InvalidCallError", err)
	}
	if len(f.scopes) != before || len(f.svcCalls) != 0 {
		t.Fatalf("exchanges %v, service calls %v: nothing may be exchanged or sent", f.scopes[before:], f.svcCalls)
	}
}

func TestCallOperationRetriesOnceOnTokenStale(t *testing.T) {
	f := newFakeNICE(t)
	f.staleOnce = true
	res, err := call(f, newClient(t, f), "dev-1")
	if err != nil || res.Status != 200 {
		t.Fatalf("result = %d, %v", res.Status, err)
	}
	if len(f.svcCalls) != 2 {
		t.Fatalf("service calls = %v, want the call once more after token_stale", f.svcCalls)
	}
}

func TestOperationsCarryTheRichFieldsOfNICE(t *testing.T) {
	f := newFakeNICE(t)
	ops, err := newClient(t, f).Operations(context.Background(), userToken(), "usr_1", "nadi")
	if err != nil {
		t.Fatal(err)
	}
	get := ops.Operations[0]
	if get.Description != "One device of the project." || get.RequestFields[0].Description != "Device ID" ||
		fmt.Sprint(get.RequestFields[1].Enum) != "[active inactive]" || !get.RequestFields[1].EnumTruncated {
		t.Fatalf("operation = %+v", get)
	}
	if del := ops.Operations[1]; del.Description != "" || del.RequestFields[0].Enum != nil {
		t.Fatalf("what NICE does not send stays empty: %+v", del)
	}
}

func TestOperationsAreAskedAgainWithTheirETag(t *testing.T) {
	f := newFakeNICE(t)
	f.opsTag = `W/"7-a"`
	c := newClient(t, f)
	first, err := c.Operations(context.Background(), userToken(), "usr_1", "nadi")
	if err != nil || len(first.Operations) != 2 {
		t.Fatalf("first = %+v, %v", first, err)
	}
	again, err := c.Operations(context.Background(), userToken(), "usr_1", "nadi")
	if err != nil || len(again.Operations) != 2 || again.Operations[0].Key != "getDevice" {
		t.Fatalf("again = %+v, %v", again, err)
	}
	if fmt.Sprint(f.opsAsked) != `[ W/"7-a"]` {
		t.Fatalf("If-None-Match sent: %q", f.opsAsked)
	}

	// Another user never reads the list of the first.
	if _, err := c.Operations(context.Background(), userToken(), "usr_2", "nadi"); err != nil || f.opsAsked[2] != "" {
		t.Fatalf("usr_2 asked with %q, %v", f.opsAsked[2], err)
	}

	// A new ETag replaces the kept list.
	f.mu.Lock()
	f.opsTag = `W/"8-b"`
	f.mu.Unlock()
	if _, err := c.Operations(context.Background(), userToken(), "usr_1", "nadi"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Operations(context.Background(), userToken(), "usr_1", "nadi"); err != nil || f.opsAsked[4] != `W/"8-b"` {
		t.Fatalf("after a change, asked with %q, %v", f.opsAsked[4], err)
	}

	// A refusal drops the kept list, so the next read asks without a tag.
	f.mu.Lock()
	f.opsGone = true
	f.mu.Unlock()
	if _, err := c.Operations(context.Background(), userToken(), "usr_1", "nadi"); err == nil {
		t.Fatal("404 is an error")
	}
	f.mu.Lock()
	f.opsGone = false
	f.mu.Unlock()
	if _, err := c.Operations(context.Background(), userToken(), "usr_1", "nadi"); err != nil || f.opsAsked[6] != "" {
		t.Fatalf("after 404, asked with %q, %v", f.opsAsked[6], err)
	}
}

func TestKeptListsEvictTheLeastRecentlyUsed(t *testing.T) {
	k := newKeptLists(3)
	for _, key := range []string{"a", "b", "c"} {
		k.put(key, credentials.ServiceOperations{ETag: key})
	}
	k.get("a")
	k.put("d", credentials.ServiceOperations{ETag: "d"})
	if _, ok := k.get("b"); ok || k.len() != 3 {
		t.Fatalf("b should be evicted; len %d", k.len())
	}
	for _, key := range []string{"a", "c", "d"} {
		if _, ok := k.get(key); !ok {
			t.Fatalf("%s should be kept", key)
		}
	}
	k = newKeptLists(maxKeptLists)
	for i := range maxKeptLists + 1 {
		k.put(fmt.Sprint(i), credentials.ServiceOperations{})
	}
	if _, ok := k.get("0"); ok || k.len() != maxKeptLists {
		t.Fatalf("entry %d evicts the oldest; len %d", maxKeptLists+1, k.len())
	}
}
