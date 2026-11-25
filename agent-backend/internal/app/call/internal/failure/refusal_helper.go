package failure

import calldto "github.com/nice-platform/example/agent-backend/internal/app/call/dto"

// NotApproved is a sensitive operation the user declined or dismissed.
func NotApproved() *calldto.Failure {
	return &calldto.Failure{Code: "not_approved", Message: "the user did not approve this operation; nothing was sent"}
}

// Invalid refuses an argument before anything is exchanged for the target.
func Invalid(why string) *calldto.Failure {
	return &calldto.Failure{Code: "invalid_argument", Message: why}
}
