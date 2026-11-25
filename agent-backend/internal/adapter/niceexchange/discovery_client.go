package niceexchange

import (
	"context"

	calldto "github.com/nice-platform/example/agent-backend/internal/app/call/dto"
)

// Services reads the service map of the user of token. The SDK reads every page with one delegated token of the
// NICE API per subject token, and keeps nothing.
func (c *Client) Services(ctx context.Context, token string) (calldto.ServiceMap, error) {
	m, err := c.x.Services(ctx, token)
	if err != nil {
		return calldto.ServiceMap{}, domainError(err)
	}
	var out calldto.ServiceMap
	for _, s := range m.Services {
		out.Services = append(out.Services, calldto.MapService{Slug: s.Slug, Name: s.Name, Description: s.Description,
			Endpoint: s.Endpoint, BasePath: s.BasePath})
	}
	for _, o := range m.Organizations {
		org := calldto.MapOrganization{ID: o.ID, Name: o.Name, Slug: o.Slug, ProjectsTruncated: o.ProjectsTruncated}
		for _, p := range o.Projects {
			org.Projects = append(org.Projects, calldto.MapProject{ID: p.ID, Name: p.Name, Slug: p.Slug, Services: p.Services})
		}
		out.Organizations = append(out.Organizations, org)
	}
	return out, nil
}

// Operations reads the operations of the service slug whose permission the user of token holds.
func (c *Client) Operations(ctx context.Context, token, slug string) (calldto.ServiceOperations, error) {
	ops, err := c.x.ServiceOperations(ctx, token, slug)
	if err != nil {
		return calldto.ServiceOperations{}, domainError(err)
	}
	out := calldto.ServiceOperations{Service: ops.Service}
	for _, o := range ops.Operations {
		op := calldto.Operation{Key: o.Key, Method: o.Method, Path: o.Path, Summary: o.Summary, Permission: o.Permission,
			Tag: o.Tag, Sensitive: o.Sensitive}
		for _, h := range o.Headers {
			op.Headers = append(op.Headers, calldto.Header{Name: h.Name, Value: h.Value})
		}
		for _, f := range o.RequestFields {
			op.RequestFields = append(op.RequestFields, calldto.RequestField{Location: f.Location, Name: f.Name, Required: f.Required, Type: f.Type})
		}
		out.Operations = append(out.Operations, op)
	}
	return out, nil
}
