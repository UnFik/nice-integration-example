package bootstrap

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nice-platform/example/service-account-client/internal/platform/config"
)

const (
	testAccount = "sa_01a17b4e-2222-7000-8000-000000000002"
	testKey     = "nice_sk_testprefix.do-not-leak-this-key"
	// tokenMark is in every token the fake NICE issues, so a test can look for a leaked token.
	tokenMark = "access-token-do-not-leak"
)

// clock is the time niceclient sees; a test moves it past the renewal time of a token.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

type refusal struct {
	status            int
	code, description string
}

func writeRefusal(w http.ResponseWriter, r refusal) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(r.status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": r.code, "error_description": r.description})
}

// fakeNICE plays the token endpoint for a Service Account, following docs/api.md, "Service Account:
// client_credentials": the id as client_id, the API key as client_secret, one service as scope, no refresh token.
type fakeNICE struct {
	srv *httptest.Server

	mu       sync.Mutex
	requests []url.Values
	issued   []string
	refuse   *refusal
}

func newFakeNICE(t *testing.T) *fakeNICE {
	f := &fakeNICE{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /oauth/token", f.token)
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeNICE) token(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, r.PostForm)
	switch {
	case f.refuse != nil:
		writeRefusal(w, *f.refuse)
	case r.PostForm.Get("grant_type") != "client_credentials":
		writeRefusal(w, refusal{400, "unsupported_grant_type", ""})
	case r.PostForm.Get("client_id") != testAccount || r.PostForm.Get("client_secret") != testKey:
		writeRefusal(w, refusal{401, "invalid_client", "client authentication failed"})
	case r.PostForm.Get("scope") != "nice:aud:nadi":
		writeRefusal(w, refusal{400, "invalid_scope", "the service is not active in the project"})
	default:
		tok := fmt.Sprintf("%s-%d", tokenMark, len(f.issued)+1)
		f.issued = append(f.issued, tok)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": tok, "token_type": "Bearer", "expires_in": 1800})
	}
}

// Refuse makes every later token request fail with r; nil lets NICE answer again.
func (f *fakeNICE) Refuse(r *refusal) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refuse = r
}

func (f *fakeNICE) Requests() []url.Values {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.requests)
}

func (f *fakeNICE) Issued() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.issued)
}

type call struct {
	Method, Path, Query string
	Header              http.Header
	Body                string
}

func (c call) Bearer() string { return strings.TrimPrefix(c.Header.Get("Authorization"), "Bearer ") }

// fakeNadi plays the device API of sharedservice-oauth/backend: a Bearer token NICE issued, the same bodies, and the
// refusals of nicetoken.
type fakeNadi struct {
	srv  *httptest.Server
	nice *fakeNICE

	mu      sync.Mutex
	calls   []call
	script  []refusal
	devices []map[string]any
}

func newFakeNadi(t *testing.T, nice *fakeNICE) *fakeNadi {
	f := &fakeNadi{nice: nice, devices: []map[string]any{
		{"id": "dev_1", "name": "Sensor Suhu Gudang", "serial_number": "SN-2026-0001", "status": "active", "location": "Gudang A", "created_by": "usr_owner"},
	}}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

// Script makes the next requests fail with these refusals, one each, before the fake answers normally again.
func (f *fakeNadi) Script(r ...refusal) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.script = append(f.script, r...)
}

func (f *fakeNadi) Calls() []call {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

func (f *fakeNadi) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	c := call{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Header: r.Header.Clone(), Body: string(body)}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, c)
	if len(f.script) > 0 {
		next := f.script[0]
		f.script = f.script[1:]
		if next.status == http.StatusUnauthorized {
			w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token", error_description="`+next.description+`"`)
		}
		writeRefusal(w, next)
		return
	}
	if !slices.Contains(f.nice.Issued(), c.Bearer()) {
		w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token", error_description="bad_signature"`)
		writeRefusal(w, refusal{401, "invalid_token", "bad_signature"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	switch r.Method + " " + r.URL.Path {
	case "GET /devices":
		_ = json.NewEncoder(w).Encode(map[string]any{"devices": f.devices})
	case "POST /devices":
		var in map[string]any
		_ = json.Unmarshal(body, &in)
		in["id"] = fmt.Sprintf("dev_%d", len(f.devices)+1)
		in["created_by"] = testAccount
		f.devices = append(f.devices, in)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(in)
	default:
		writeRefusal(w, refusal{404, "not_found", ""})
	}
}

// breaker is the network between the client and one host; a test cuts it to make that host unreachable.
type breaker struct {
	host string
	down atomic.Bool
}

func (b *breaker) RoundTrip(r *http.Request) (*http.Response, error) {
	if b.down.Load() && r.URL.Host == b.host {
		return nil, errors.New("connection refused")
	}
	return http.DefaultTransport.RoundTrip(r)
}

// buffer is a bytes.Buffer the log goroutines and the command can write to at once.
type buffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *buffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *buffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

// scriptWaiter stands in for the pause of watch: it does not wait, runs the step of each round, and interrupts the
// watch after stop rounds (never when stop is 0).
type scriptWaiter struct {
	steps  map[int]func()
	stop   int
	cancel context.CancelFunc
	round  int
}

func (w *scriptWaiter) Wait(ctx context.Context, _ time.Duration) error {
	w.round++
	if step := w.steps[w.round]; step != nil {
		step()
	}
	if w.round == w.stop {
		w.cancel()
	}
	return ctx.Err()
}

// world is the client with everything around it faked.
type world struct {
	t        *testing.T
	nice     *fakeNICE
	nadi     *fakeNadi
	clock    *clock
	niceNet  *breaker
	nadiNet  *breaker
	env      map[string]string
	out, err *buffer
}

func newWorld(t *testing.T) *world {
	nice := newFakeNICE(t)
	nadi := newFakeNadi(t, nice)
	w := &world{t: t, nice: nice, nadi: nadi, clock: &clock{t: time.Date(2026, 11, 19, 9, 0, 0, 0, time.UTC)}, out: &buffer{}, err: &buffer{}}
	w.niceNet = &breaker{host: strings.TrimPrefix(nice.srv.URL, "http://")}
	w.nadiNet = &breaker{host: strings.TrimPrefix(nadi.srv.URL, "http://")}
	w.env = map[string]string{config.EnvIssuer: nice.srv.URL, config.EnvAccountID: testAccount, config.EnvAPIKey: testKey,
		config.EnvNadiURL: nadi.srv.URL, config.EnvWatchInterval: "1s"}
	// Every test ends with the same check: whatever happened, no secret left the places it belongs.
	t.Cleanup(w.assertNoSecretLeaked)
	return w
}

type transport struct{ nice, nadi *breaker }

func (t transport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Host == t.nice.host {
		return t.nice.RoundTrip(r)
	}
	return t.nadi.RoundTrip(r)
}

// run executes one command with the real assembly and returns the exit code.
func (w *world) run(ctx context.Context, waiter *scriptWaiter, args ...string) int {
	lookup := func(k string) (string, bool) {
		v, ok := w.env[k]
		return v, ok
	}
	if waiter != nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithCancel(ctx)
		defer cancel()
		waiter.cancel = cancel
	}
	o := Options{HTTP: &http.Client{Transport: transport{w.niceNet, w.nadiNet}, Timeout: 5 * time.Second},
		Now: w.clock.Now, Tick: 2 * time.Millisecond, Jitter: -1}
	if waiter != nil {
		o.Waiter = waiter
	}
	log := slog.New(slog.NewTextHandler(w.err, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return RunWith(ctx, args, lookup, w.t.TempDir()+"/.env", w.out, w.err, log, o)
}

// assertNoSecretLeaked checks that the API key reached NICE only, and that neither the key nor a token was printed.
func (w *world) assertNoSecretLeaked() {
	w.t.Helper()
	for _, c := range w.nadi.Calls() {
		dump := fmt.Sprint(c.Method, c.Path, c.Query, c.Header, c.Body)
		if strings.Contains(dump, testKey) {
			w.t.Errorf("the API key was sent to nadi: %s %s", c.Method, c.Path)
		}
	}
	for name, text := range map[string]string{"stdout": w.out.String(), "stderr": w.err.String()} {
		if strings.Contains(text, testKey) {
			w.t.Errorf("the API key is in %s", name)
		}
		if strings.Contains(text, tokenMark) {
			w.t.Errorf("a token is in %s", name)
		}
	}
}

// eventually waits for cond, which a background renewal of the SDK makes true.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		if cond() {
			return
		}
	}
	t.Fatalf("timed out waiting for %s", what)
}
