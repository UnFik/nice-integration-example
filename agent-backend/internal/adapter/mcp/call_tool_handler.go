package mcpadapter

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nice-platform/example/agent-backend/internal/adapter/mcp/internal/caller"
	"github.com/nice-platform/example/agent-backend/internal/adapter/mcp/internal/mapper"
	calldto "github.com/nice-platform/example/agent-backend/internal/app/call/dto"
	"github.com/nice-platform/example/agent-backend/internal/domain"
)

// callTool runs one call for the user of the MCP request. A nil output with a result is an error the model reads.
func (h *Handlers) callTool(ctx context.Context, req *mcp.CallToolRequest, t domain.Target, where calldto.Where, method, path string, body any) (*mcp.CallToolResult, any, error) {
	c, err := caller.From(req)
	if err != nil {
		return mapper.FailureResult(&calldto.Failure{Code: "unauthenticated", Message: err.Error()}), nil, nil
	}
	out, failure := h.Calls.ExecuteCall(ctx, c, t, where, method, path, body)
	if failure != nil {
		return mapper.FailureResult(failure), nil, nil
	}
	return nil, out, nil
}
