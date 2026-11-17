package domain

import "time"

// Context is the organization and project a browser session works in. Only
// the IDs are kept: names are read from NICE when shown.
type Context struct {
	Org     string
	Project string
}

// User is the NICE account of a session, read once at sign-in.
type User struct {
	ID    string
	Name  string
	Email string
}

// Session is a browser session of the UI. ID is the value of the cookie;
// Subject is another random value under which niceclient keeps the refresh
// token, so the cookie never names the stored tokens.
type Session struct {
	ID      string
	Subject string
	// Pending is a sign-in that was started and not completed: it has no
	// tokens and only lets the callback finish.
	Pending bool
	User    User
	// Context is nil until the user has an active organization and project.
	Context   *Context
	CSRFToken string
	Created   time.Time
	LastUsed  time.Time
}
