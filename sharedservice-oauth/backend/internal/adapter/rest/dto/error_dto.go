// Package restdto holds the REST bodies shared by every resource.
package restdto

// ErrorBody has the shape niceauth/verify uses for its own refusals, so a client
// reads every error of the service the same way.
type ErrorBody struct {
	Error            string `json:"error" example:"invalid_argument"`
	ErrorDescription string `json:"error_description,omitempty" example:"name"`
} // @name Error
