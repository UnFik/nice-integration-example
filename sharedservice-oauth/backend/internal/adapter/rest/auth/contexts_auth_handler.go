package auth

import (
	"errors"
	"net/http"

	authrestdto "github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/auth/dto"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/internal/reply"
	sessionusecase "github.com/nice-platform/example/sharedservice-oauth/backend/internal/app/session/usecase"
)

// Contexts answers the organizations and projects of the user, read from NICE
// on every call because their names can change there.
func (h *Handler) Contexts(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.session(w, r)
	if !ok {
		return
	}
	cs, err := h.Sessions.Contexts(r.Context(), sess)
	var refused *sessionusecase.RefusedError
	switch {
	case errors.As(err, &refused):
		reply.Error(w, http.StatusBadGateway, "contexts_unavailable", refused.Code)
		return
	case err != nil:
		h.fail(w, err)
		return
	}
	out := authrestdto.Contexts{CurrentOrganizationID: cs.CurrentOrganizationID, Organizations: []authrestdto.Organization{}}
	for _, o := range cs.Organizations {
		org := authrestdto.Organization{ID: o.ID, Name: o.Name, Slug: o.Slug, ProjectsTruncated: o.ProjectsTruncated, Projects: []authrestdto.Project{}}
		for _, p := range o.Projects {
			org.Projects = append(org.Projects, authrestdto.Project{ID: p.ID, Name: p.Name, Slug: p.Slug})
		}
		out.Organizations = append(out.Organizations, org)
	}
	reply.JSON(w, http.StatusOK, out)
}
