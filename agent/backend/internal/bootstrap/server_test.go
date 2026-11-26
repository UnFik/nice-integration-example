package bootstrap

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/nice-platform/example/agent/backend/docs"
	mcpadapter "github.com/nice-platform/example/agent/backend/internal/adapter/mcp"
	"github.com/nice-platform/example/agent/backend/internal/domain"
)

const initialize = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"curl","version":"1"}}}`

func errorOf(t *testing.T, resp *http.Response) (string, string) {
	t.Helper()
	var body struct {
		Error       string `json:"error"`
		Description string `json:"error_description"`
	}
	raw, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("body is not a JSON error: %s", raw)
	}
	if strings.Contains(string(raw), "eyJ") {
		t.Fatalf("the error body echoes a token: %s", raw)
	}
	return body.Error, body.Description
}

func TestMCPWithoutTokenPointsToTheResourceMetadata(t *testing.T) {
	e := newTestEnv(t)
	resp := e.post(mcpadapter.MCPPath, "", initialize)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
	want := `resource_metadata="http://agent.test/.well-known/oauth-protected-resource"`
	if h := resp.Header.Get("WWW-Authenticate"); !strings.HasPrefix(h, "Bearer ") || !strings.Contains(h, want) {
		t.Fatalf("WWW-Authenticate = %q, want Bearer with %s", h, want)
	}
	if code, desc := errorOf(t, resp); code != "invalid_token" || desc != "missing_token" {
		t.Fatalf("error = %s %s", code, desc)
	}
}

func TestMCPRefusesATokenForAnotherService(t *testing.T) {
	e := newTestEnv(t)
	tok := sign(t, userClaims(e.issuer, "nadi", orgA, prjP, domain.PermUse))
	resp := e.post(mcpadapter.MCPPath, tok, initialize)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
	h := resp.Header.Get("WWW-Authenticate")
	if !strings.Contains(h, `error_description="wrong_audience"`) || !strings.Contains(h, "resource_metadata=") {
		t.Fatalf("WWW-Authenticate = %q", h)
	}
	if _, desc := errorOf(t, resp); desc != "wrong_audience" {
		t.Fatalf("error_description = %s, want wrong_audience", desc)
	}
}

func TestMCPRefusesATokenWithoutThePermission(t *testing.T) {
	e := newTestEnv(t)
	tok := sign(t, userClaims(e.issuer, domain.ServiceSlug, orgA, prjP))
	resp := e.post(mcpadapter.MCPPath, tok, initialize)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
	if code, desc := errorOf(t, resp); code != "insufficient_scope" || desc != "permission_denied" {
		t.Fatalf("error = %s %s", code, desc)
	}
	if h := resp.Header.Get("WWW-Authenticate"); !strings.Contains(h, `error="insufficient_scope"`) || !strings.Contains(h, "resource_metadata=") {
		t.Fatalf("WWW-Authenticate = %q", h)
	}
}

func TestMCPAcceptsATokenOfTheAgent(t *testing.T) {
	e := newTestEnv(t)
	s := e.connect(e.userToken(orgA, prjP))
	if _, err := s.ListTools(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
}

func TestResourceMetadataNamesNICE(t *testing.T) {
	e := newTestEnv(t)
	for _, path := range []string{"/.well-known/oauth-protected-resource", "/.well-known/oauth-protected-resource/mcp"} {
		resp, err := http.Get(e.agent.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		var md struct {
			Resource             string   `json:"resource"`
			AuthorizationServers []string `json:"authorization_servers"`
			ScopesSupported      []string `json:"scopes_supported"`
			BearerMethods        []string `json:"bearer_methods_supported"`
		}
		err = json.NewDecoder(resp.Body).Decode(&md)
		_ = resp.Body.Close()
		if err != nil || resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: %d %v", path, resp.StatusCode, err)
		}
		if md.Resource != "http://agent.test/mcp" || len(md.AuthorizationServers) != 1 || md.AuthorizationServers[0] != e.issuer {
			t.Fatalf("%s: metadata = %+v", path, md)
		}
		if !strings.Contains(strings.Join(md.ScopesSupported, " "), "nice:aud:agen") ||
			!strings.Contains(strings.Join(md.ScopesSupported, " "), "openid") {
			t.Fatalf("%s: scopes_supported = %v", path, md.ScopesSupported)
		}
	}
}

func TestPublicEndpoints(t *testing.T) {
	e := newTestEnv(t)
	resp, err := http.Get(e.agent.URL + "/healthz")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz: %v %v", resp, err)
	}
	_ = resp.Body.Close()
	resp, err = http.Get(e.agent.URL + "/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || string(raw) != string(docs.OpenAPI) {
		t.Fatalf("openapi.json: %d", resp.StatusCode)
	}
}
