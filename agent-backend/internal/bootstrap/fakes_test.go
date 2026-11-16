package bootstrap

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nice-platform/example/agent-backend/internal/domain"

	"github.com/nice-platform/nice/pkg/nicetoken"
)

// fakeNICE plays the parts of NICE the agent talks to: the token endpoint (token exchange only) and the proxy
// /pengiriman/... in front of a fake pengiriman. Its rules follow docs/api.md, "Token exchange: backend atas nama
// user", in the small: confidential actor, subject for agen, target among the dependencies, organization of the
// user, project of that organization.
type fakeNICE struct {
	t          *testing.T
	srv        *httptest.Server
	subject    *nicetoken.Verifier
	pengiriman *fakeService

	mu sync.Mutex
	// projects are the organizations the user belongs to and their projects.
	projects map[string][]string
	// perms are the permissions an exchanged token carries per target.
	perms map[string][]string
	// refuse forces an answer of the token endpoint for a target.
	refuse    map[string]refusal
	exchanges []exchangeCall
	// onExchange runs inside every exchange before it is answered; tests use it to hold exchanges in flight.
	onExchange func(aud string)
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
			"inventaris": {"inventaris.items.read", "inventaris.items.create"},
			"pengiriman": {"pengiriman.shipments.read", "pengiriman.shipments.create"},
		},
		refuse: map[string]refusal{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /oauth/token", f.token)
	mux.HandleFunc("/pengiriman/", f.proxy)
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	v, err := nicetoken.New(nicetoken.Config{Issuer: f.srv.URL, Service: domain.ServiceSlug, StaticJWKS: testJWKS(t), Log: slog.New(slog.DiscardHandler)})
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
	perms, dependency := f.perms[call.Audience]
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
	case !dependency:
		tokenError(w, 400, "invalid_scope", "the service is not a dependency of agen")
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

// proxy stands for the NICE proxy of the api_key service pengiriman: it refuses with the error body of NICE, and
// forwards the rest to the fake pengiriman.
func (f *fakeNICE) proxy(w http.ResponseWriter, r *http.Request) {
	r2 := r.Clone(r.Context())
	r2.URL.Path = strings.TrimPrefix(r.URL.Path, "/pengiriman")
	f.pengiriman.niceErrors = true
	f.pengiriman.ServeHTTP(w, r2)
}

// fakeService is a nice_token service with one collection, verified with nicetoken as inventaris does.
type fakeService struct {
	t          *testing.T
	slug       string
	collection string
	srv        *httptest.Server
	v          *nicetoken.Verifier
	niceErrors bool

	mu        sync.Mutex
	received  []map[string]any
	staleOnce bool
	created   []map[string]any
	// hold, when set, makes each request wait on it.
	hold chan struct{}
}

func newFakeService(t *testing.T, issuer, slug string) *fakeService {
	s := &fakeService{t: t, slug: slug, collection: map[string]string{"inventaris": "items", "pengiriman": "shipments"}[slug]}
	v, err := nicetoken.New(nicetoken.Config{Issuer: issuer, Service: slug, StaticJWKS: testJWKS(t), Log: slog.New(slog.DiscardHandler)})
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

func (s *fakeService) StaleOnce() { s.mu.Lock(); s.staleOnce = true; s.mu.Unlock() }

func (s *fakeService) fail(w http.ResponseWriter, status int, code, description string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if s.niceErrors {
		_ = json.NewEncoder(w).Encode(map[string]any{"code": status, "error_code": description, "message": "permission denied"})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "error_description": description})
}

func (s *fakeService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	c := readClaims(s.t, tok)
	s.mu.Lock()
	s.received = append(s.received, c)
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
		s.fail(w, 401, "invalid_token", err.(*nicetoken.Error).Description)
		return
	}
	if r.URL.Path != "/"+s.collection {
		s.fail(w, 404, "not_found", "")
		return
	}
	perm := s.slug + "." + s.collection + ".read"
	if r.Method == http.MethodPost {
		perm = s.slug + "." + s.collection + ".create"
	}
	if !claims.Has(perm) {
		s.fail(w, 403, "insufficient_scope", "permission_denied")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodGet {
		s.mu.Lock()
		list := slices.Clone(s.created)
		s.mu.Unlock()
		if list == nil {
			list = []map[string]any{}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{s.collection: list, "org": claims.OrgID, "project": claims.ProjectID})
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
	s.created = append(s.created, in)
	s.mu.Unlock()
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(in)
}
