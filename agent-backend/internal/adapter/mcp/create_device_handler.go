package mcpadapter

import (
	"context"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	calldto "github.com/nice-platform/example/agent-backend/internal/app/call/dto"
	"github.com/nice-platform/example/agent-backend/internal/domain"
)

// CreateDevice is the tool nadi_create_device.
func (h *Handlers) CreateDevice(ctx context.Context, req *mcp.CallToolRequest, in createDeviceInput) (*mcp.CallToolResult, any, error) {
	body := map[string]any{"name": in.Name, "serial_number": in.SerialNumber, "status": in.Status}
	// nadi defaults an absent location to empty; send it only when the model gave one.
	if in.Location != "" {
		body["location"] = in.Location
	}
	return h.callTool(ctx, req, domain.Nadi, calldto.Where{Org: in.OrganizationID, Project: in.ProjectID}, http.MethodPost, "/devices", body)
}
