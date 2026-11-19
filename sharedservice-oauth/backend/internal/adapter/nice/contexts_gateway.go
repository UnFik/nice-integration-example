package nicegateway

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"

	"github.com/ABA-Developer/nice-auth-sdk/credentials"
	sessiondto "github.com/nice-platform/example/sharedservice-oauth/backend/internal/app/session/dto"
	sessionusecase "github.com/nice-platform/example/sharedservice-oauth/backend/internal/app/session/usecase"
)

// maxContextPages bounds the pages read, so a NICE that keeps answering a
// next page cannot hold a request forever.
const maxContextPages = 100

type contextsPage struct {
	CurrentOrganizationID string `json:"current_organization_id"`
	Organizations         []struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Slug     string `json:"slug"`
		Projects []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Slug string `json:"slug"`
		} `json:"projects"`
		ProjectsTruncated bool `json:"projects_truncated"`
	} `json:"organizations"`
	Page struct {
		NextPageToken string `json:"next_page_token"`
	} `json:"page"`
}

// Contexts trades the sign-in token for a delegated token of the NICE API and
// reads every page of GET /api/v1/me/contexts with it. The sign-in token works
// before any project is known, so the picker is filled right after sign-in.
func (g *Gateway) Contexts(ctx context.Context, signIn string) (sessiondto.Contexts, error) {
	var out sessiondto.Contexts
	token := ""
	for range maxContextPages {
		q := url.Values{"page_size": {"50"}}
		if token != "" {
			q.Set("page_token", token)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.Issuer+"/api/v1/me/contexts?"+q.Encode(), nil)
		if err != nil {
			return sessiondto.Contexts{}, err
		}
		resp, err := g.Exchange.DoAs(ctx, signIn, credentials.Key{Audience: "nice"}, req)
		if err != nil {
			return sessiondto.Contexts{}, exchangeError(err)
		}
		page, err := readPage(resp)
		if err != nil {
			return sessiondto.Contexts{}, err
		}
		if out.CurrentOrganizationID == "" {
			out.CurrentOrganizationID = page.CurrentOrganizationID
		}
		for _, o := range page.Organizations {
			org := sessiondto.Organization{ID: o.ID, Name: o.Name, Slug: o.Slug, ProjectsTruncated: o.ProjectsTruncated,
				Projects: make([]sessiondto.Project, 0, len(o.Projects))}
			for _, p := range o.Projects {
				org.Projects = append(org.Projects, sessiondto.Project{ID: p.ID, Name: p.Name, Slug: p.Slug})
			}
			out.Organizations = append(out.Organizations, org)
		}
		if page.Page.NextPageToken == "" {
			return out, nil
		}
		token = page.Page.NextPageToken
	}
	return sessiondto.Contexts{}, &sessionusecase.UnavailableError{Err: errors.New("GET /api/v1/me/contexts did not end")}
}

func readPage(resp *http.Response) (contextsPage, error) {
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return contextsPage{}, answerError(resp)
	}
	var page contextsPage
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&page); err != nil {
		return contextsPage{}, &sessionusecase.UnavailableError{Err: err}
	}
	return page, nil
}
