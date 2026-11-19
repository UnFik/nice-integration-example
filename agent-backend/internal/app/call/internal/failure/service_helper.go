package failure

import (
	"encoding/json"
	"fmt"
	"strings"

	calldto "github.com/nice-platform/example/agent-backend/internal/app/call/dto"
	"github.com/nice-platform/example/agent-backend/internal/domain"
)

// HTTP statuses the messages tell apart; the use case does not import net/http.
const (
	statusBadRequest   = 400
	statusUnauthorized = 401
	statusNotFound     = 404
	statusServerError  = 500
)

// Service explains a refusal of a service. Only the code and description are read from the body, in the shape of
// niceauth/verify ({error, error_description}).
func Service(t domain.Target, status int, raw []byte) *calldto.Failure {
	var body struct {
		Error       string `json:"error"`
		Description string `json:"error_description"`
	}
	_ = json.Unmarshal(raw, &body)
	f := &calldto.Failure{Code: safe(body.Error), Description: safe(body.Description)}
	if f.Code == "" {
		f.Code = fmt.Sprintf("http_%d", status)
	}
	reason := f.Code + " " + f.Description
	answered := fmt.Sprintf("%s answered %d", t, status)
	switch {
	case strings.Contains(reason, "permission_denied"):
		f.Message = answered + ": the user does not hold the permission for this operation in that project"
	case status == statusUnauthorized:
		f.Message = answered + ": it refused the exchanged token; get a new user token and call the tool again"
	case status == statusBadRequest:
		f.Message = answered + ": check the arguments"
	case status == statusNotFound:
		f.Message = answered + ": not found"
	case status >= statusServerError:
		f.Message = answered + ": the service is unavailable; try again later"
	default:
		f.Message = answered
	}
	return f
}
