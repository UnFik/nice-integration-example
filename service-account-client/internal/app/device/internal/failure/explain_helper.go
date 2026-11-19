// Package failure turns the errors of a call into a Failure the operator can act on: what happened and which step
// of the README setup checklist to look at.
package failure

import (
	"errors"
	"fmt"

	devicedto "github.com/nice-platform/example/service-account-client/internal/app/device/dto"
	"github.com/nice-platform/example/service-account-client/internal/domain"
)

// The steps of the README setup checklist. scripts/smoke.sh names the same numbers.
const (
	StepService = "1"
	StepAccount = "2"
	StepBinding = "3"
	StepKey     = "4"
)

// The statuses of nadi the client tells apart. The use cases do not import net/http, so they are named here.
const (
	statusBadRequest      = 400
	statusUnauthorized    = 401
	statusForbidden       = 403
	statusUnprocessable   = 422
	statusTooManyRequests = 429
)

// Explain names err in the terms of the operator.
func Explain(err error) *devicedto.Failure {
	var grant *domain.GrantError
	var svc *domain.ServiceError
	switch {
	case errors.As(err, &grant):
		return explainGrant(grant)
	case errors.As(err, &svc):
		return explainService(svc)
	case errors.Is(err, domain.ErrTemporary):
		return &devicedto.Failure{Code: "nice_unavailable", Temporary: true,
			Message: "NICE did not answer the token request: check that NICE is running and that NICE_ISSUER is its address"}
	case errors.Is(err, domain.ErrUnreachable):
		return &devicedto.Failure{Code: "service_unavailable", Temporary: true, Step: StepService,
			Message: "nadi did not answer: check that sharedservice-oauth/backend is running and that NADI_URL is its address"}
	case errors.Is(err, domain.ErrBadAnswer):
		return &devicedto.Failure{Code: "bad_answer", Step: StepService,
			Message: "the answer is not that of nadi: check that NADI_URL is the address of sharedservice-oauth/backend"}
	default:
		return &devicedto.Failure{Code: "internal", Message: "the call failed before it reached NICE or nadi"}
	}
}

// Input refuses a field before any request is made.
func Input(field, why string) *devicedto.Failure {
	return &devicedto.Failure{Code: "invalid_argument", Description: field, Message: why}
}

func explainGrant(g *domain.GrantError) *devicedto.Failure {
	f := &devicedto.Failure{Code: safe(g.Code), Description: safe(g.Description)}
	if f.Code == "" {
		f.Code = "token_refused"
	}
	switch {
	case g.Permanent:
		f.Step = StepKey
		f.Message = "NICE refused the API key for good: the key is wrong, revoked or expired, the Service Account is disabled, " +
			"or NICE_SA_ID is not the owner of NICE_SA_API_KEY. Replace the key and start the client again"
	case g.Code == "invalid_scope":
		f.Step = StepAccount
		f.Message = "NICE issues no token for nadi to this Service Account: nadi is not entitled to its organization, " +
			"or not active in its project"
	default:
		f.Message = "NICE refused the token request"
	}
	return f
}

func explainService(e *domain.ServiceError) *devicedto.Failure {
	f := &devicedto.Failure{Code: safe(e.Code), Description: safe(e.Description)}
	if f.Code == "" {
		f.Code = fmt.Sprintf("http_%d", e.Status)
	}
	switch {
	case e.Status == statusUnauthorized:
		f.Message = unauthorized(e.Description)
		if e.Description == "wrong_issuer" || e.Description == "wrong_audience" {
			f.Step = StepService
		}
	case e.Status == statusForbidden:
		f.Step = StepBinding
		f.Message = forbidden(e.Description)
	case e.Status == statusBadRequest || e.Status == statusUnprocessable:
		f.Message = "nadi refused the fields of the request"
	case e.Status == statusTooManyRequests || e.Status >= 500:
		f.Temporary = true
		f.Message = "nadi cannot answer now; try again later"
	default:
		f.Message = fmt.Sprintf("nadi answered HTTP %d", e.Status)
	}
	return f
}

func unauthorized(reason string) string {
	switch reason {
	case "token_stale":
		return "the access of the Service Account changed and nadi still refuses the new token: try again in a moment"
	case "wrong_issuer":
		return "nadi trusts another NICE: NICE_ISSUER here and in sharedservice-oauth/backend/.env must be the same"
	case "wrong_audience":
		return "nadi is not registered under the slug nadi, so the token is for another service"
	default:
		return "nadi does not accept the token NICE issued"
	}
}

func forbidden(reason string) string {
	switch reason {
	case "permission_denied":
		return "the token lacks the permission of this command (" + domain.PermRead + " for list and watch, " + domain.PermCreate +
			" for create): bind the Service Account to a nadi role that holds it, such as nadi.editor"
	case "org_rejected":
		return "nadi does not accept the organization of the Service Account"
	default:
		return "nadi refused the request for the Service Account"
	}
}
