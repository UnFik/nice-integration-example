package callusecase

import (
	"context"

	calldto "github.com/nice-platform/example/agent/backend/internal/app/call/dto"
	"github.com/nice-platform/example/agent/backend/internal/app/call/internal/failure"
	"github.com/nice-platform/example/agent/backend/internal/domain"
)

// ListOperations returns the operations of the service slug whose permission the user of c holds.
func (s *Service) ListOperations(ctx context.Context, c calldto.Caller, slug string) (*calldto.OperationsView, *calldto.Failure) {
	if !domain.ValidServiceSlug(slug) {
		return nil, failure.Invalid("service must be the slug of a service from list_services")
	}
	ops, err := s.Discoverer.Operations(ctx, c.Token, slug)
	if err != nil {
		return nil, failure.Discovery(domain.Target(slug), err, s.ClientVars)
	}
	out := &calldto.OperationsView{Service: slug, Operations: []calldto.OperationView{}}
	for _, o := range ops.Operations {
		fields := o.RequestFields
		if fields == nil {
			fields = []calldto.RequestField{}
		}
		out.Operations = append(out.Operations, calldto.OperationView{Key: o.Key, Method: o.Method, Path: o.Path,
			Summary: o.Summary, Permission: o.Permission, Tag: o.Tag, Sensitive: o.Sensitive, RequestFields: fields})
	}
	return out, nil
}
