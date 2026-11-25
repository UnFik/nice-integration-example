package mcpadapter

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nice-platform/example/agent-backend/internal/adapter/mcp/internal/caller"
	"github.com/nice-platform/example/agent-backend/internal/adapter/mcp/internal/mapper"
)

// ListOperations is the tool list_operations.
func (h *Handlers) ListOperations(ctx context.Context, req *mcp.CallToolRequest, in listOperationsInput) (*mcp.CallToolResult, any, error) {
	c, err := caller.From(req)
	if err != nil {
		return mapper.Unauthenticated(err), nil, nil
	}
	out, f := h.Calls.ListOperations(ctx, c, in.Service)
	if f != nil {
		return mapper.FailureResult(f), nil, nil
	}
	return nil, out, nil
}
