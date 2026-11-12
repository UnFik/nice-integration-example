package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nice-platform/nice/pkg/niceclient"
)

// target is a service the tools call. Its slug is the audience of the exchanged token.
type target string

const (
	inventaris target = "inventaris"
	pengiriman target = "pengiriman"
)

// contextArgs is the organization and project a tool call names; empty keeps those of the user's token.
type contextArgs struct {
	Org     string
	Project string
}

const maxBody = 1 << 20

// call sends one request to t on behalf of the user of c and turns the answer into a tool result: the decoded JSON
// body on success, or a result marked as an error that names the code and says what it means. Neither the token nor
// the raw body of a refusal ever reaches the result.
func (a *agent) call(ctx context.Context, c caller, t target, where contextArgs, method, path string, body any) (*mcp.CallToolResult, any) {
	key, bad := keyFor(c, t, where)
	if bad != nil {
		return bad.result(), nil
	}
	var rd io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return (&toolError{Code: "invalid_argument", Message: "the arguments cannot be encoded"}).result(), nil
		}
		// A bytes.Reader gives the request a GetBody, so DoAs can send it again after token_stale.
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, a.bases[t]+path, rd)
	if err != nil {
		return (&toolError{Code: "internal", Message: "the request to " + string(t) + " cannot be built"}).result(), nil
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.callAs(ctx, c, key, req)
	if err != nil {
		te := exchangeError(t, err)
		a.log.Warn("tool call failed", slog.String("service", string(t)), slog.String("user", c.claims.Subject), slog.String("code", te.Code))
		return te.result(), nil
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return (&toolError{Code: "service_unavailable", Message: string(t) + " did not finish its answer; try again"}).result(), nil
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		te := serviceError(t, resp.StatusCode, raw)
		a.log.Info("service refused", slog.String("service", string(t)), slog.Int("status", resp.StatusCode), slog.String("code", te.Code))
		return te.result(), nil
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return (&toolError{Code: "bad_answer", Message: string(t) + " answered something other than a JSON object"}).result(), nil
	}
	return nil, out
}

// callAs is the only place that touches OAuth: it sends req with a token exchanged for key on behalf of the user of
// c. DoAs keeps the token per (user, organization, project, service), shares one exchange among parallel calls of one
// key, and sends the request once more after a fresh exchange when the service answers 401 token_stale.
func (a *agent) callAs(ctx context.Context, c caller, key niceclient.Key, req *http.Request) (*http.Response, error) {
	return a.x.DoAs(ctx, c.token, key, req)
}

var (
	orgID     = regexp.MustCompile(`^org_[A-Za-z0-9-]{1,64}$`)
	projectID = regexp.MustCompile(`^prj_[A-Za-z0-9-]{1,64}$`)
)

// keyFor is the exchange key of a call. The IDs come from the model, so they are checked before they become scopes:
// a space would add a scope of its choosing.
func keyFor(c caller, t target, where contextArgs) (niceclient.Key, *toolError) {
	if where.Org != "" && !orgID.MatchString(where.Org) {
		return niceclient.Key{}, &toolError{Code: "invalid_argument", Message: "organization_id must look like org_<id>"}
	}
	if where.Project != "" && !projectID.MatchString(where.Project) {
		return niceclient.Key{}, &toolError{Code: "invalid_argument", Message: "project_id must look like prj_<id>"}
	}
	if where.Org == c.claims.OrgID {
		where.Org = ""
	}
	// The agent never picks a project in another organization: the user names it.
	if where.Org != "" && where.Project == "" {
		return niceclient.Key{}, &toolError{Code: "invalid_argument",
			Message: "project_id is required with an organization_id other than the one of the user's token"}
	}
	return niceclient.Key{Org: where.Org, Project: where.Project, Audience: string(t)}, nil
}

// toolError is a failure as the model and the user read it.
type toolError struct {
	Code        string
	Description string
	Message     string
}

func (e *toolError) result() *mcp.CallToolResult {
	text := e.Code
	if e.Description != "" && e.Description != e.Code {
		text += " (" + e.Description + ")"
	}
	text += ": " + e.Message
	r := &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}, IsError: true}
	return r
}

// safe keeps a code or description from NICE or a service only when it is short printable text without anything
// shaped like a token.
var safeText = regexp.MustCompile(`^[\x20-\x7E]{1,200}$`)

func safe(s string) string {
	s = strings.TrimSpace(s)
	if !safeText.MatchString(s) || strings.Contains(s, "eyJ") {
		return ""
	}
	return s
}

// exchangeError explains a failed exchange, or a service that could not be reached.
func exchangeError(t target, err error) *toolError {
	var grant *niceclient.GrantError
	var urlErr *url.Error
	switch {
	case errors.As(err, &grant):
		te := &toolError{Code: safe(grant.Code), Description: safe(grant.Description)}
		if te.Code == "" {
			te.Code = "exchange_refused"
		}
		switch grant.Code {
		case "access_denied":
			te.Message = fmt.Sprintf("NICE refused a token for %s in that organization: the user is not an active member of it, the organization is not active, or it does not use %s or agen", t, t)
		case "invalid_scope":
			te.Message = fmt.Sprintf("NICE refused a token for %s in that context: %s is not a dependency of agen or not active, or the project is not active in that organization or does not use %s", t, t, t)
		case "invalid_grant":
			te.Message = invalidGrant(grant.Description)
		case "invalid_client", "unauthorized_client":
			te.Message = fmt.Sprintf("NICE refused the agent's own client: the operator must check %s and %s (a confidential service client bound to agen)", envClientID, envClientSecret)
		default:
			te.Message = "NICE refused the token exchange"
		}
		return te
	case niceclient.IsTemporary(err):
		return &toolError{Code: "nice_unavailable", Message: "NICE did not answer the token exchange; try again later"}
	case errors.As(err, &urlErr):
		return &toolError{Code: "service_unavailable", Message: string(t) + " did not answer; try again later"}
	default:
		return &toolError{Code: "exchange_failed", Message: "no token for " + string(t) + " could be obtained for this user"}
	}
}

func invalidGrant(description string) string {
	switch {
	case strings.HasPrefix(description, "subject_token_expiring"):
		return "the user's token expires within a minute: refresh it in the MCP client and call the tool again"
	case strings.HasPrefix(description, "token_stale"):
		return "the user's access changed after the token was issued: get a new token and call the tool again"
	default:
		return "NICE does not accept the user's token for an exchange (it is not a valid user token for agen, or the user or session ended): sign in again"
	}
}

// serviceError explains a refusal of a service. Only the code and description are read from the body, in the shape
// of nicetoken ({error, error_description}) or of the NICE proxy ({error_code, message}).
func serviceError(t target, status int, raw []byte) *toolError {
	var body struct {
		Error       string `json:"error"`
		Description string `json:"error_description"`
		ErrorCode   string `json:"error_code"`
		Message     string `json:"message"`
	}
	_ = json.Unmarshal(raw, &body)
	te := &toolError{Code: safe(body.Error), Description: safe(body.Description)}
	if te.Code == "" {
		te.Code, te.Description = safe(body.ErrorCode), safe(body.Message)
	}
	if te.Code == "" {
		te.Code = fmt.Sprintf("http_%d", status)
	}
	reason := te.Code + " " + te.Description
	answered := fmt.Sprintf("%s answered %d", t, status)
	switch {
	case strings.Contains(reason, "permission_denied"):
		te.Message = answered + ": the user does not hold the permission for this operation in that project"
	case strings.Contains(reason, "service_account_required"):
		te.Message = answered + ": the NICE proxy accepts only Service Accounts for " + string(t) + " because its keys are mirrored per account"
	case status == http.StatusUnauthorized:
		te.Message = answered + ": it refused the exchanged token; get a new user token and call the tool again"
	case status == http.StatusBadRequest:
		te.Message = answered + ": check the arguments"
	case status == http.StatusNotFound:
		te.Message = answered + ": not found"
	case status >= 500:
		te.Message = answered + ": the service is unavailable; try again later"
	default:
		te.Message = answered
	}
	return te
}
