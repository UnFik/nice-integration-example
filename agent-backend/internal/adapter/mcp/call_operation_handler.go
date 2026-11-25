package mcpadapter

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nice-platform/example/agent-backend/internal/adapter/mcp/internal/approval"
	"github.com/nice-platform/example/agent-backend/internal/adapter/mcp/internal/caller"
	"github.com/nice-platform/example/agent-backend/internal/adapter/mcp/internal/mapper"
	calldto "github.com/nice-platform/example/agent-backend/internal/app/call/dto"
)

// CallOperation is the tool call_operation. A sensitive operation comes back to the client as an elicitation with a
// state; the client asks the user and calls the tool again with the answer and the state.
func (h *Handlers) CallOperation(ctx context.Context, req *mcp.CallToolRequest, in callOperationInput) (*mcp.CallToolResult, any, error) {
	c, err := caller.From(req)
	if err != nil {
		return mapper.Unauthenticated(err), nil, nil
	}
	out, ask, f := h.Calls.CallOperation(ctx, c, calldto.OperationInput{
		Service: in.Service, Operation: in.Operation,
		Where:      calldto.Where{Org: in.OrganizationID, Project: in.ProjectID},
		PathParams: in.PathParams, Query: in.Query, Body: in.Body,
	}, approval.Answer(req))
	switch {
	case f != nil:
		return mapper.FailureResult(f), nil, nil
	case ask != nil:
		res, ok := approval.Ask(req, ask)
		if !ok {
			return mapper.ApprovalUnavailable(), nil, nil
		}
		return res, nil, nil
	}
	return nil, out, nil
}
