package mcpadapter

import (
	"context"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	calldto "github.com/nice-platform/example/agent-backend/internal/app/call/dto"
	"github.com/nice-platform/example/agent-backend/internal/domain"
)

// ListShipments is the tool pengiriman_list_shipments.
func (h *Handlers) ListShipments(ctx context.Context, req *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, any, error) {
	return h.callTool(ctx, req, domain.Pengiriman, calldto.Where{Org: in.OrganizationID, Project: in.ProjectID}, http.MethodGet, "/shipments", nil)
}
