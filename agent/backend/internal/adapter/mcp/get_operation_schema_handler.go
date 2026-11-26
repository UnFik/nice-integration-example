package mcpadapter

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nice-platform/example/agent/backend/internal/adapter/mcp/internal/caller"
	"github.com/nice-platform/example/agent/backend/internal/adapter/mcp/internal/mapper"
)

// GetOperationSchema is the tool get_operation_schema.
func (h *Handlers) GetOperationSchema(ctx context.Context, req *mcp.CallToolRequest, in getOperationSchemaInput) (*mcp.CallToolResult, any, error) {
	c, err := caller.From(req)
	if err != nil {
		return mapper.Unauthenticated(err), nil, nil
	}
	out, f := h.Calls.GetOperationSchemas(ctx, c, in.Service, in.Operations)
	if f != nil {
		return mapper.FailureResult(f), nil, nil
	}
	return nil, out, nil
}
