package nicegateway

import (
	"context"
	"errors"

	"github.com/ABA-Developer/nice-auth-sdk/credentials"
	sessiondto "github.com/nice-platform/example/sharedservice-oauth/backend/internal/app/session/dto"
	sessionusecase "github.com/nice-platform/example/sharedservice-oauth/backend/internal/app/session/usecase"
)

// Contexts reads the contexts of the user with the sign-in token through the
// SDK, which exchanges it for a delegated token of the NICE API and reads every
// page of GET /api/v1/me/contexts. The sign-in token works before any project
// is known, so the picker is filled right after sign-in.
func (g *Gateway) Contexts(ctx context.Context, signIn string) (sessiondto.Contexts, error) {
	cs, err := g.Exchange.Contexts(ctx, signIn)
	if err != nil {
		return sessiondto.Contexts{}, contextsError(err)
	}
	out := sessiondto.Contexts{CurrentOrganizationID: cs.CurrentOrganization, Organizations: make([]sessiondto.Organization, 0, len(cs.Organizations))}
	for _, o := range cs.Organizations {
		org := sessiondto.Organization{ID: o.ID, Name: o.Name, Slug: o.Slug, ActAs: o.ActAs,
			ProjectsTruncated: o.ProjectsTruncated, Projects: make([]sessiondto.Project, 0, len(o.Projects))}
		for _, p := range o.Projects {
			org.Projects = append(org.Projects, sessiondto.Project{ID: p.ID, Name: p.Name, Slug: p.Slug, Services: p.Services})
		}
		out.Organizations = append(out.Organizations, org)
	}
	return out, nil
}

// contextsError maps a failure of the contexts. It never ends the session:
// invalid_grant there is about the subject token, not the refresh token.
func contextsError(err error) error {
	var api *credentials.APIError
	if !credentials.IsTemporary(err) && errors.As(err, &api) {
		code := api.Code
		if code == "" {
			code = "nice_api_error"
		}
		return &sessionusecase.RefusedError{Code: code, Description: api.Message}
	}
	return exchangeError(err)
}
