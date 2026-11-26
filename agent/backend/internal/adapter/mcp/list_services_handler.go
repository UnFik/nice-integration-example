package mcpadapter

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nice-platform/example/agent/backend/internal/adapter/mcp/internal/caller"
	"github.com/nice-platform/example/agent/backend/internal/adapter/mcp/internal/mapper"
)

// ListServices is the tool list_services.
func (h *Handlers) ListServices(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	c, err := caller.From(req)
	if err != nil {
		return mapper.Unauthenticated(err), nil, nil
	}
	out, f := h.Calls.ListServices(ctx, c)
	if f != nil {
		return mapper.FailureResult(f), nil, nil
	}
	return nil, out, nil
}
