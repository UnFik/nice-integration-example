// Package authrestdto holds the JSON bodies of /auth/*. They are for the UI of
// this example only and are not part of the OpenAPI document of the service.
package authrestdto

type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type Context struct {
	Org     string `json:"org"`
	Project string `json:"project"`
}

// Session never holds a token: only what the UI shows and the CSRF token it
// sends back on every write.
type Session struct {
	User      User     `json:"user"`
	Context   *Context `json:"context"`
	Perms     []string `json:"perms"`
	CSRFToken string   `json:"csrf_token"`
	// ContextError says why the token of the active context could not be had.
	ContextError string `json:"context_error"`
}

type Project struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type Organization struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Slug              string    `json:"slug"`
	ActAs             bool      `json:"act_as"`
	Projects          []Project `json:"projects"`
	ProjectsTruncated bool      `json:"projects_truncated"`
}

type Contexts struct {
	CurrentOrganizationID string         `json:"current_organization_id"`
	Organizations         []Organization `json:"organizations"`
}

// Logout tells the UI where the browser ends its NICE session too ("sign
// out"). For "switch account" the UI goes to /auth/login?prompt=select_account
// instead, which keeps the NICE session and shows its account chooser.
type Logout struct {
	EndSessionURL string `json:"end_session_url"`
}
