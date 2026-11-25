package mcpadapter

import (
	"context"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	calldto "github.com/nice-platform/example/agent-backend/internal/app/call/dto"
	"github.com/nice-platform/example/agent-backend/internal/domain"
)

// ListDevices is the tool nadi_list_devices.
func (h *Handlers) ListDevices(ctx context.Context, req *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, any, error) {
	return h.callTool(ctx, req, domain.Nadi, calldto.Where{Org: in.OrganizationID, Project: in.ProjectID}, http.MethodGet, "/devices", nil)
}
