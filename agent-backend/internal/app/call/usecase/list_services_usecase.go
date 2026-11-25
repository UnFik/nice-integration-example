package callusecase

import (
	"context"

	calldto "github.com/nice-platform/example/agent-backend/internal/app/call/dto"
	"github.com/nice-platform/example/agent-backend/internal/app/call/internal/failure"
)

// ListServices returns where the user of c may call which service, without the addresses of the services.
func (s *Service) ListServices(ctx context.Context, c calldto.Caller) (*calldto.ServicesView, *calldto.Failure) {
	m, err := s.Discoverer.Services(ctx, c.Token)
	if err != nil {
		return nil, failure.Discovery("", err, s.ClientVars)
	}
	out := &calldto.ServicesView{Services: []calldto.ServiceView{}, Organizations: []calldto.OrganizationView{}}
	for _, svc := range m.Services {
		out.Services = append(out.Services, calldto.ServiceView{Slug: svc.Slug, Name: svc.Name, Description: svc.Description})
	}
	for _, o := range m.Organizations {
		org := calldto.OrganizationView{ID: o.ID, Name: o.Name, Projects: []calldto.ProjectView{}, Incomplete: o.ProjectsTruncated}
		for _, p := range o.Projects {
			org.Projects = append(org.Projects, calldto.ProjectView{ID: p.ID, Name: p.Name, Services: p.Services})
		}
		out.Organizations = append(out.Organizations, org)
	}
	return out, nil
}
