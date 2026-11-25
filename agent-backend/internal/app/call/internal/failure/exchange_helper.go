package failure

import (
	"errors"
	"fmt"
	"strings"

	calldto "github.com/nice-platform/example/agent-backend/internal/app/call/dto"
	"github.com/nice-platform/example/agent-backend/internal/domain"
)

// Exchange explains a failed exchange, or a service that could not be reached. clientVars names the settings of the
// agent's own client.
func Exchange(t domain.Target, err error, clientVars string) *calldto.Failure {
	var grant *domain.GrantError
	var bad *domain.InvalidCallError
	switch {
	case errors.As(err, &bad):
		return &calldto.Failure{Code: "invalid_argument", Message: "the call was not sent: " + orDefault(safe(bad.Reason), "the arguments do not fit the operation")}
	case errors.As(err, &grant):
		f := &calldto.Failure{Code: safe(grant.Code), Description: safe(grant.Description)}
		if f.Code == "" {
			f.Code = "exchange_refused"
		}
		switch grant.Code {
		case "access_denied":
			f.Message = fmt.Sprintf("NICE refused a token for %s in that organization: the user is not an active member of it, the organization is not active, or it does not use %s or agen", t, t)
		case "invalid_scope":
			f.Message = fmt.Sprintf("NICE refused a token for %s in that context: %s is not active, or the project is not active in that organization or does not use %s", t, t, t)
		case "invalid_grant":
			f.Message = invalidGrant(grant.Description)
		case "invalid_client", "unauthorized_client":
			f.Message = fmt.Sprintf("NICE refused the agent's own client: the operator must check %s (a confidential service client bound to agen)", clientVars)
		default:
			f.Message = "NICE refused the token exchange"
		}
		return f
	case errors.Is(err, domain.ErrTemporary):
		return &calldto.Failure{Code: "nice_unavailable", Message: "NICE did not answer the token exchange; try again later"}
	case errors.Is(err, domain.ErrUnreachable):
		return &calldto.Failure{Code: "service_unavailable", Message: string(t) + " did not answer; try again later"}
	default:
		return &calldto.Failure{Code: "exchange_failed", Message: "no token for " + string(t) + " could be obtained for this user"}
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

// Discovery explains a failure to read the service map (slug empty) or the operations of the service slug.
func Discovery(slug domain.Target, err error, clientVars string) *calldto.Failure {
	var api *domain.APIError
	if !errors.As(err, &api) {
		f := Exchange("the NICE API", err, clientVars)
		if errors.Is(err, domain.ErrUnreachable) {
			f.Code, f.Message = "nice_unavailable", "NICE did not answer; try again later"
		}
		return f
	}
	f := &calldto.Failure{Code: safe(api.Code)}
	if f.Code == "" {
		f.Code = fmt.Sprintf("http_%d", api.Status)
	}
	switch {
	case api.Status == statusNotFound && slug == "":
		f.Code = "discovery_unavailable"
		f.Message = "NICE does not list the services of a user: it is older than the caller service discovery; the operator must upgrade NICE"
	case api.Status == statusNotFound:
		f.Code = "service_not_allowed"
		f.Message = fmt.Sprintf("the user may not call %s in any project, or it is not a nice_token service", slug)
	case api.Code == "delegation_not_allowed":
		f.Code = "token_stale_after_upgrade"
		f.Message = "NICE refused the delegated token issued before it was upgraded: get a new user token and call the tool again"
	default:
		f.Message = fmt.Sprintf("NICE answered %d to the discovery request", api.Status)
	}
	return f
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
