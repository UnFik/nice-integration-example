package mcpadapter

import (
	"context"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	calldto "github.com/nice-platform/example/agent-backend/internal/app/call/dto"
	"github.com/nice-platform/example/agent-backend/internal/domain"
)

// CreateItem is the tool inventaris_create_item.
func (h *Handlers) CreateItem(ctx context.Context, req *mcp.CallToolRequest, in createItemInput) (*mcp.CallToolResult, any, error) {
	body := map[string]any{"name": in.Name, "quantity": in.Quantity}
	return h.callTool(ctx, req, domain.Inventaris, calldto.Where{Org: in.OrganizationID, Project: in.ProjectID}, http.MethodPost, "/items", body)
}
