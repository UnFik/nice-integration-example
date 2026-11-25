package bootstrap

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nice-platform/example/agent-backend/internal/domain"

	"github.com/ABA-Developer/nice-auth-sdk/verify"
)

// fakeNICE plays the part of NICE the agent talks to: the token endpoint (token exchange only) and the caller service
// discovery (GET /api/v1/me/services and the operations of nadi). Its rules follow docs/api.md, "Token exchange:
// backend atas nama user", in the small: confidential actor, subject for agen, an active target, organization of the
// user, project of that organization. The token carries the user's permissions in the target project: NICE sets no
// ceiling for the actor. Discovery answers only a delegated token for the NICE API (aud nice).
type fakeNICE struct {
	t       *testing.T
	srv     *httptest.Server
	subject *verify.Verifier

	mu sync.Mutex
	// projects are the organizations the user belongs to and their projects.
	projects map[string][]string
	// perms are the user's permissions per target; a target without an entry is not an active service.
	perms map[string][]string
	// refuse forces an answer of the token endpoint for a target.
	refuse    map[string]refusal
	exchanges []exchangeCall
	// onExchange runs inside every exchange before it is answered; tests use it to hold exchanges in flight.
	onExchange func(aud string)
	// endpoint is the catalog endpoint of nadi; noDiscovery makes NICE older than the discovery operations.
	endpoint    string
	noDiscovery bool
	mapReads    int
}

type refusal struct {
	status            int
	code, description string
}

type exchangeCall struct {
	Scope, Org, Project, Audience string
}

func newFakeNICE(t *testing.T) *fakeNICE {
	f := &fakeNICE{
		t:        t,
		projects: map[string][]string{orgA: {prjP}, orgB: {prjQ}},
		perms: map[string][]string{
			"nadi": {"nadi.devices.read", "nadi.devices.create", "nadi.devices.delete"},
			"nice": {"nice.me.services.read"},
		},
		refuse: map[string]refusal{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /oauth/token", f.token)
	mux.HandleFunc("GET /api/v1/me/services", f.services)
	mux.HandleFunc("GET /api/v1/me/services/{slug}/operations", f.operations)
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	v, err := verify.New(verify.Config{Issuer: f.srv.URL, Service: domain.ServiceSlug, StaticJWKS: testJWKS(t), Log: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatal(err)
	}
	f.subject = v
	return f
}

func (f *fakeNICE) URL() string { return f.srv.URL }

func (f *fakeNICE) Exchanges() []exchangeCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.exchanges)
}

// ExchangesFor are the exchanges for one audience.
func (f *fakeNICE) ExchangesFor(aud string) []exchangeCall {
	var out []exchangeCall
	for _, ex := range f.Exchanges() {
		if ex.Audience == aud {
			out = append(out, ex)
		}
	}
	return out
}

func (f *fakeNICE) apiError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error_code": code, "message": code})
}

// delegated reports whether the request carries a token of the user for the NICE API.
func (f *fakeNICE) delegated(w http.ResponseWriter, r *http.Request) bool {
	f.mu.Lock()
	old := f.noDiscovery
	f.mu.Unlock()
	if old {
		f.apiError(w, http.StatusNotFound, "not_found")
		return false
	}
	c := readClaims(f.t, strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if aud, _ := c["aud"].([]any); len(aud) != 1 || aud[0] != "nice" {
		f.apiError(w, http.StatusUnauthorized, "unauthenticated")
		return false
	}
	return true
}

func (f *fakeNICE) services(w http.ResponseWriter, r *http.Request) {
	if !f.delegated(w, r) {
		return
	}
	f.mu.Lock()
	f.mapReads++
	endpoint := f.endpoint
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprintf(w, `{"services":[{"slug":"nadi","name":"NADI","description":"IoT devices","endpoint":%q,"base_path":""}],
		"organizations":[
		  {"id":%q,"name":"Org A","slug":"a","projects":[{"id":%q,"name":"Demo","slug":"demo","services":["nadi"]}]},
		  {"id":%q,"name":"Org B","slug":"b","projects":[{"id":%q,"name":"Lokasi Q","slug":"q","services":["nadi"]}]}],
		"page":{"next_page_token":""}}`, endpoint, orgA, prjP, orgB, prjQ)
}

// nadiOperations are the operations of nadi in the catalog; deleteDevice is sensitive, as in sharedservice-oauth.
const nadiOperations = `{"service":"nadi","operations":[
	{"key":"listDevices","method":"GET","path":"/devices","summary":"List devices","permission":"nadi.devices.read",
	 "request_fields":[{"location":"query","name":"status","type":"string"}]},
	{"key":"createDevice","method":"POST","path":"/devices","summary":"Register a device","permission":"nadi.devices.create",
	 "request_fields":[{"location":"body","name":"name","required":true},{"location":"body","name":"serial_number","required":true},
	   {"location":"body","name":"status","required":true},{"location":"body","name":"location"}]},
	{"key":"getDevice","method":"GET","path":"/devices/{id}","summary":"Get a device","permission":"nadi.devices.read",
	 "request_fields":[{"location":"path","name":"id","required":true}]},
	{"key":"deleteDevice","method":"DELETE","path":"/devices/{id}","summary":"Delete a device","permission":"nadi.devices.delete",
	 "sensitive":true,"request_fields":[{"location":"path","name":"id","required":true}]}]}`

func (f *fakeNICE) operations(w http.ResponseWriter, r *http.Request) {
	if !f.delegated(w, r) {
		return
	}
	if r.PathValue("slug") != "nadi" {
		f.apiError(w, http.StatusNotFound, "not_found")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprint(w, nadiOperations)
}

func (f *fakeNICE) Refuse(aud string, status int, code, description string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refuse[aud] = refusal{status, code, description}
}

func tokenError(w http.ResponseWriter, status int, code, description string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "error_description": description})
}

func (f *fakeNICE) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		tokenError(w, 400, "invalid_request", "form")
		return
	}
	id, secret, ok := r.BasicAuth()
	if !ok {
		id, secret = r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
	}
	if id != testClientID || secret != testSecret {
		tokenError(w, 401, "invalid_client", "client authentication failed")
		return
	}
	if r.PostForm.Get("grant_type") != "urn:ietf:params:oauth:grant-type:token-exchange" ||
		r.PostForm.Get("subject_token_type") != "urn:ietf:params:oauth:token-type:access_token" {
		tokenError(w, 400, "invalid_request", "not a token exchange")
		return
	}
	sub, err := f.subject.Verify(r.Context(), r.PostForm.Get("subject_token"))
	if err != nil {
		tokenError(w, 400, "invalid_grant", "subject_invalid: the subject token is not a valid token of the user for agen")
		return
	}
	var aud, org, prj []string
	for _, s := range strings.Fields(r.PostForm.Get("scope")) {
		switch {
		case strings.HasPrefix(s, "nice:aud:"):
			aud = append(aud, strings.TrimPrefix(s, "nice:aud:"))
		case strings.HasPrefix(s, "nice:org:"):
			org = append(org, strings.TrimPrefix(s, "nice:org:"))
		case strings.HasPrefix(s, "nice:prj:"):
			prj = append(prj, strings.TrimPrefix(s, "nice:prj:"))
		}
	}
	if len(aud) != 1 || len(org) > 1 || len(prj) > 1 {
		tokenError(w, 400, "invalid_scope", "exactly one nice:aud, at most one nice:org and nice:prj")
		return
	}
	call := exchangeCall{Scope: r.PostForm.Get("scope"), Audience: aud[0], Org: sub.OrgID, Project: sub.ProjectID}
	if len(org) == 1 {
		call.Org = org[0]
		if call.Org != sub.OrgID {
			call.Project = ""
		}
	}
	if len(prj) == 1 {
		call.Project = prj[0]
	}

	f.mu.Lock()
	f.exchanges = append(f.exchanges, call)
	forced, refused := f.refuse[call.Audience]
	perms, active := f.perms[call.Audience]
	projects, member := f.projects[call.Org]
	hook := f.onExchange
	f.mu.Unlock()
	if hook != nil {
		hook(call.Audience)
	}

	switch {
	case refused:
		tokenError(w, forced.status, forced.code, forced.description)
		return
	case !active:
		tokenError(w, 400, "invalid_scope", "the service is not active")
		return
	case !member:
		tokenError(w, 400, "access_denied", "the user is not a member of the organization")
		return
	case call.Project == "" || !slices.Contains(projects, call.Project):
		tokenError(w, 400, "invalid_scope", "the project cannot be used")
		return
	}
	exp := time.Now().Add(30 * time.Minute).Unix()
	exp = min(exp, sub.ExpiresAt)
	tok := sign(f.t, map[string]any{
		"iss": f.srv.URL, "sub": sub.Subject, "aud": []string{call.Audience}, "azp": id, "act": map[string]string{"sub": id},
		"iat": time.Now().Unix(), "nbf": time.Now().Unix(), "exp": exp,
		"nice_org_id": call.Org, "nice_principal_type": "user", "prj": call.Project, "perms": perms, "pv": 1,
	})
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"access_token": tok, "issued_token_type": "urn:ietf:params:oauth:token-type:access_token",
		"token_type": "Bearer", "expires_in": exp - time.Now().Unix(), "scope": call.Scope,
	})
}

// fakeService is a nice_token service with one collection, verified with niceauth/verify as nadi does.
type fakeService struct {
	t          *testing.T
	slug       string
	collection string
	srv        *httptest.Server
	v          *verify.Verifier

	mu        sync.Mutex
	received  []map[string]any
	requests  []string
	staleOnce bool
	created   []map[string]any
	// hold, when set, makes each request wait on it.
	hold chan struct{}
}

func newFakeService(t *testing.T, issuer, slug string) *fakeService {
	s := &fakeService{t: t, slug: slug, collection: "devices"}
	v, err := verify.New(verify.Config{Issuer: issuer, Service: slug, StaticJWKS: testJWKS(t), Log: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatal(err)
	}
	s.v = v
	s.srv = httptest.NewServer(s)
	t.Cleanup(s.srv.Close)
	return s
}

func (s *fakeService) Received() []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.received)
}

// Requests are the method and path of every request, in order.
func (s *fakeService) Requests() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.requests)
}

func (s *fakeService) StaleOnce() { s.mu.Lock(); s.staleOnce = true; s.mu.Unlock() }

func (s *fakeService) fail(w http.ResponseWriter, status int, code, description string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "error_description": description})
}

func (s *fakeService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	c := readClaims(s.t, tok)
	s.mu.Lock()
	s.received = append(s.received, c)
	s.requests = append(s.requests, r.Method+" "+r.URL.Path)
	stale := s.staleOnce
	s.staleOnce = false
	hold := s.hold
	s.mu.Unlock()
	if hold != nil {
		<-hold
	}
	if stale {
		w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token", error_description="token_stale"`)
		s.fail(w, 401, "invalid_token", "token_stale")
		return
	}
	claims, err := s.v.Verify(r.Context(), tok)
	if err != nil {
		s.fail(w, 401, "invalid_token", err.(*verify.Error).Description)
		return
	}
	id, item := strings.CutPrefix(r.URL.Path, "/"+s.collection+"/")
	if r.URL.Path != "/"+s.collection && !item {
		s.fail(w, 404, "not_found", "")
		return
	}
	perm := s.slug + "." + s.collection + ".read"
	switch r.Method {
	case http.MethodPost:
		perm = s.slug + "." + s.collection + ".create"
	case http.MethodDelete:
		perm = s.slug + "." + s.collection + ".delete"
	}
	if !claims.Has(perm) {
		s.fail(w, 403, "insufficient_scope", "permission_denied")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if item {
		s.mu.Lock()
		defer s.mu.Unlock()
		i := slices.IndexFunc(s.created, func(d map[string]any) bool { return d["id"] == id })
		switch {
		case i < 0:
			s.fail(w, 404, "not_found", "device not found")
		case r.Method == http.MethodDelete:
			s.created = slices.Delete(s.created, i, i+1)
			w.WriteHeader(http.StatusNoContent)
		default:
			_ = json.NewEncoder(w).Encode(s.created[i])
		}
		return
	}
	if r.Method == http.MethodGet {
		s.mu.Lock()
		list := slices.Clone(s.created)
		s.mu.Unlock()
		if list == nil {
			list = []map[string]any{}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{s.collection: list, "org": claims.OrgID, "project": claims.ProjectID,
			"status": r.URL.Query().Get("status")})
		return
	}
	var in map[string]any
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		s.fail(w, 400, "invalid_argument", "body")
		return
	}
	in["created_by"] = claims.Subject
	if claims.Actor != "" {
		in["acted_by"] = claims.Actor
	}
	s.mu.Lock()
	in["id"] = fmt.Sprintf("dev-%d", len(s.created)+1)
	s.created = append(s.created, in)
	s.mu.Unlock()
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(in)
}
