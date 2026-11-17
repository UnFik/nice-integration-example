// Package sessiondto holds the data that crosses the boundary of the session use cases.
package sessiondto

// UserInfo is what /oauth/userinfo tells about the user.
type UserInfo struct {
	Name  string
	Email string
}

// Contexts are the organizations and projects where the user may use the
// service, as GET /api/v1/me/contexts of NICE answers them over all pages.
type Contexts struct {
	CurrentOrganizationID string
	Organizations         []Organization
}

type Organization struct {
	ID                string
	Name              string
	Slug              string
	Projects          []Project
	ProjectsTruncated bool
}

type Project struct {
	ID   string
	Name string
	Slug string
}
