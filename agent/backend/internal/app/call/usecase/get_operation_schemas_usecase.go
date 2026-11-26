package callusecase

import (
	"context"
	"errors"
	"fmt"

	calldto "github.com/nice-platform/example/agent/backend/internal/app/call/dto"
	"github.com/nice-platform/example/agent/backend/internal/app/call/internal/failure"
	"github.com/nice-platform/example/agent/backend/internal/domain"
)

// maxSchemaKeys keeps one answer of get_operation_schema small in the context of the model: a schema can be 64 KiB.
const maxSchemaKeys = 10

// GetOperationSchemas returns the input and response schemas of the operations keys of the service slug that the
// user of c may call, and the other keys as missing.
func (s *Service) GetOperationSchemas(ctx context.Context, c calldto.Caller, slug string, keys []string) (*calldto.SchemasView, *calldto.Failure) {
	if !domain.ValidServiceSlug(slug) {
		return nil, failure.Invalid("service must be the slug of a service from list_services")
	}
	if len(keys) == 0 || len(keys) > maxSchemaKeys {
		return nil, failure.Invalid(fmt.Sprintf("operations must hold 1 to %d keys from list_operations", maxSchemaKeys))
	}
	for _, k := range keys {
		if k == "" || len(k) > maxOperationKey {
			return nil, failure.Invalid("operations must hold keys from list_operations")
		}
	}
	got, err := s.Discoverer.OperationSchemas(ctx, c.Token, slug, keys)
	if err != nil {
		if notFound(err) {
			// NICE answers 404 both for a service out of reach and before it served schemas; the list tells which.
			if _, opsErr := s.Discoverer.Operations(ctx, c.Token, c.Subject, slug); opsErr == nil {
				return nil, schemasUnavailable()
			}
		}
		return nil, failure.Discovery(domain.Target(slug), err, s.ClientVars)
	}
	out := &calldto.SchemasView{Service: slug, Schemas: got.Schemas, Missing: got.Missing}
	if out.Schemas == nil {
		out.Schemas = []calldto.OperationSchema{}
	}
	if out.Missing == nil {
		out.Missing = []string{}
	}
	return out, nil
}

func notFound(err error) bool {
	var api *domain.APIError
	return errors.As(err, &api) && api.Status == 404
}

func schemasUnavailable() *calldto.Failure {
	return &calldto.Failure{Code: "schemas_unavailable",
		Message: "NICE does not serve operation schemas: it is older than the operation schemas; use the request fields of list_operations"}
}
