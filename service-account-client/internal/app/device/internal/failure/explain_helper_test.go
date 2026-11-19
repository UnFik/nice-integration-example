package failure

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/nice-platform/example/service-account-client/internal/domain"
)

func TestExplain(t *testing.T) {
	cases := []struct {
		name      string
		err       error
		code      string
		step      string
		temporary bool
		says      string
	}{
		{"key refused", &domain.GrantError{Code: "invalid_client", Description: "client authentication failed", Permanent: true},
			"invalid_client", StepKey, false, "API key"},
		{"scope refused", &domain.GrantError{Code: "invalid_scope", Description: "nadi is not active"}, "invalid_scope", StepAccount, false, "not entitled"},
		{"another refusal", &domain.GrantError{Code: "invalid_request"}, "invalid_request", "", false, "refused the token request"},
		{"NICE down", fmt.Errorf("wrapped: %w", domain.ErrTemporary), "nice_unavailable", "", true, "NICE_ISSUER"},
		{"nadi down", domain.ErrUnreachable, "service_unavailable", StepService, true, "NADI_URL"},
		{"not nadi", domain.ErrBadAnswer, "bad_answer", StepService, false, "NADI_URL"},
		{"bad fields", &domain.ServiceError{Status: 400, Code: "invalid_argument", Description: "serial_number"}, "invalid_argument", "", false, "fields"},
		{"stale twice", &domain.ServiceError{Status: 401, Code: "invalid_token", Description: "token_stale"}, "invalid_token", "", false, "access of the Service Account changed"},
		{"another NICE", &domain.ServiceError{Status: 401, Code: "invalid_token", Description: "wrong_issuer"}, "invalid_token", StepService, false, "NICE_ISSUER"},
		{"another slug", &domain.ServiceError{Status: 401, Code: "invalid_token", Description: "wrong_audience"}, "invalid_token", StepService, false, "slug"},
		{"no permission", &domain.ServiceError{Status: 403, Code: "insufficient_scope", Description: "permission_denied"},
			"insufficient_scope", StepBinding, false, domain.PermCreate},
		{"organization refused", &domain.ServiceError{Status: 403, Code: "insufficient_scope", Description: "org_rejected"}, "insufficient_scope", StepBinding, false, "organization"},
		{"nadi broken", &domain.ServiceError{Status: 503, Code: "temporarily_unavailable"}, "temporarily_unavailable", "", true, "try again"},
		{"nadi busy", &domain.ServiceError{Status: 429}, "http_429", "", true, "try again"},
		{"no body", &domain.ServiceError{Status: 404}, "http_404", "", false, "HTTP 404"},
		{"unknown", errors.New("boom"), "internal", "", false, "before it reached"},
	}
	for _, c := range cases {
		f := Explain(c.err)
		if f.Code != c.code || f.Step != c.step || f.Temporary != c.temporary || !strings.Contains(f.Message, c.says) {
			t.Errorf("%s: %+v", c.name, *f)
		}
	}
}

func TestExplainKeepsTextOnOneBoundedLine(t *testing.T) {
	f := Explain(&domain.ServiceError{Status: 400, Code: "bad\ncode", Description: strings.Repeat("x", 5000)})
	if strings.ContainsAny(f.Code+f.Description, "\n\r") || len(f.Description) > maxText+3 {
		t.Fatalf("code = %q, description of %d bytes", f.Code, len(f.Description))
	}
}

func TestInput(t *testing.T) {
	f := Input("status", "the status must be one of active")
	if f.Code != "invalid_argument" || f.Description != "status" || f.Temporary {
		t.Fatalf("%+v", *f)
	}
}
