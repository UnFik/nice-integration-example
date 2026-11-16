package mcpadapter

import (
	"context"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	calldto "github.com/nice-platform/example/agent-backend/internal/app/call/dto"
	"github.com/nice-platform/example/agent-backend/internal/domain"
)

// CreateShipment is the tool pengiriman_create_shipment.
func (h *Handlers) CreateShipment(ctx context.Context, req *mcp.CallToolRequest, in createShipmentInput) (*mcp.CallToolResult, any, error) {
	body := map[string]any{"destination": in.Destination}
	return h.callTool(ctx, req, domain.Pengiriman, calldto.Where{Org: in.OrganizationID, Project: in.ProjectID}, http.MethodPost, "/shipments", body)
}
